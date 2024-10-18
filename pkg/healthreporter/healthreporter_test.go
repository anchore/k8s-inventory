package healthreporter

import (
	"fmt"
	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	jstime "github.com/anchore/k8s-inventory/internal/time"
	"github.com/anchore/k8s-inventory/pkg/integration"
	"github.com/google/uuid"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"reflect"
	"testing"
	"time"
)

const mutexLocked = int64(1 << iota) // mutex is locked

var (
	now        = time.Date(2024, 10, 4, 10, 11, 12, 0, time.Local)
	timestamps = []time.Time{now.Add(time.Millisecond * 10), now.Add(time.Millisecond * 20), now.Add(time.Millisecond * 30)}
	uuids      = []uuid.UUID{uuid.New(), uuid.New()}

	reportInfo = InventoryReportInfo{
		ReportTimestamp:     now.UTC().Format(time.RFC3339),
		Account:             "testAccount",
		SentAsUser:          "testAccountUser",
		BatchSize:           1,
		LastSuccessfulIndex: 1,
		HasErrors:           false,
		Batches: []BatchInfo{
			{
				BatchIndex:    0,
				SendTimestamp: jstime.Datetime{Time: time.Now().UTC()},
				Error:         "",
			},
		},
	}
	reportInfoExpired = InventoryReportInfo{
		ReportTimestamp:     now.Add(time.Second * (-3800)).UTC().Format(time.RFC3339),
		Account:             "testAccount2",
		SentAsUser:          "testAccount2User",
		BatchSize:           1,
		LastSuccessfulIndex: 1,
		HasErrors:           false,
		Batches: []BatchInfo{
			{
				BatchIndex:    0,
				SendTimestamp: jstime.Datetime{Time: time.Now().UTC()},
				Error:         "",
			},
		},
	}
)

func TestSendHealthReport(t *testing.T) {
	defer gock.Off()

	integrationUUID := uuid.New().String()
	postURL := fmt.Sprintf("/v2/system/integrations/%s/health-report", integrationUUID)
	type want struct {
		healthReport *HealthReport
		err          error
	}
	tests := []struct {
		name string
		want want
	}{
		{
			name: "successful health report",
			want: want{
				healthReport: &HealthReport{
					UUID:            uuids[0].String(),
					ProtocolVersion: 1,
					Timestamp:       jstime.Datetime{Time: timestamps[1].UTC()},
					Uptime:          &jstime.Duration{Duration: time.Millisecond * 20},
					HealthData: HealthData{
						Type:    healthDataType,
						Version: healthDataVersion,
						Errors:  make(HealthReportErrors, 0),
						AccountK8sInventoryReports: AccountK8SInventoryReports{
							reportInfo.Account: reportInfo,
						},
					},
					HealthReportInterval: 60,
				},
				err: nil,
			},
		},
		{
			name: "failed health report",
			want: want{
				healthReport: nil,
				err: &anchore.APIClientError{
					HTTPStatusCode: http.StatusUnauthorized,
					Message:        "401 Unauthorized response from Anchore (during health report)",
					Path:           postURL,
					Method:         "POST",
				},
			},
		},
	}
	for _, tt := range tests {
		cfg := config.Application{
			AnchoreDetails: config.AnchoreInfo{
				URL:  "https://ancho.re",
				User: "admin",
			},
			PollingIntervalSeconds:      30 * 60,
			HealthReportIntervalSeconds: 60,
		}
		integrationInstance := &integration.Integration{
			UUID:                 integrationUUID,
			StartedAt:            jstime.Datetime{Time: now.UTC()},
			Uptime:               &jstime.Duration{Duration: time.Millisecond * 20},
			HealthReportInterval: 60,
		}
		gatedReportInfo := GetGatedReportInfo()
		gatedReportInfo.AccountInventoryReports["testAccount"] = reportInfo
		i := 0
		newUUIDMock := func() uuid.UUID {
			_uuid := uuids[i]
			i++
			return _uuid
		}
		j := 0
		nowMock := func() time.Time {
			timestamp := timestamps[j]
			j++
			return timestamp
		}
		switch tt.name {
		case "successful health report":
			gock.New("https://ancho.re").
				Post(postURL).
				Reply(200)
		case "failed health report":
			gock.New("https://ancho.re").
				Post(postURL).
				Reply(http.StatusUnauthorized)
		}
		t.Run(tt.name, func(t *testing.T) {
			result, resultErr := sendHealthReport(&cfg, integrationInstance, gatedReportInfo, newUUIDMock, nowMock)
			if tt.want.err != nil {
				assert.Equal(t, tt.want.err, resultErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, resultErr)
				assert.Equal(t, tt.want.healthReport, result)
			}
		})
	}
}

func TestGetAccountReportInfoNoBlockingWhenObtainingLockRemovesExpired(t *testing.T) {
	gatedReportInfo := GatedReportInfo{
		AccountInventoryReports: make(AccountK8SInventoryReports, 2),
	}
	gatedReportInfo.AccountInventoryReports[reportInfo.Account] = reportInfo
	gatedReportInfo.AccountInventoryReports[reportInfoExpired.Account] = reportInfoExpired

	cfg := config.Application{
		PollingIntervalSeconds: 30 * 60,
	}

	nowMock := func() time.Time {
		return now
	}

	result := GetAccountReportInfoNoBlocking(&gatedReportInfo, &cfg, nowMock)
	assert.Equal(t, len(result), 1)
	assert.Contains(t, result, reportInfo.Account)
	assert.Equal(t, len(gatedReportInfo.AccountInventoryReports), 1)
	assert.Contains(t, gatedReportInfo.AccountInventoryReports, reportInfo.Account)
}

func TestGetAccountReportInfoBlockingWhenNotObtainingLockExpiredUnaffected(t *testing.T) {
	gatedReportInfo := GatedReportInfo{
		AccountInventoryReports: make(AccountK8SInventoryReports, 2),
	}
	gatedReportInfo.AccountInventoryReports[reportInfo.Account] = reportInfo
	gatedReportInfo.AccountInventoryReports[reportInfoExpired.Account] = reportInfoExpired
	gatedReportInfo.AccessGate.Lock()

	cfg := config.Application{
		PollingIntervalSeconds: 3 * 60,
	}

	nowMock := func() time.Time {
		return now
	}

	result := GetAccountReportInfoNoBlocking(&gatedReportInfo, &cfg, nowMock)
	assert.Equal(t, len(result), 0)
	assert.Equal(t, len(gatedReportInfo.AccountInventoryReports), 2)
	assert.Contains(t, gatedReportInfo.AccountInventoryReports, reportInfo.Account)
	assert.Contains(t, gatedReportInfo.AccountInventoryReports, reportInfoExpired.Account)
	// check mutex is still locked after operation
	mutexState := reflect.ValueOf(&gatedReportInfo.AccessGate).Elem().FieldByName("w").FieldByName("state")
	assert.Equal(t, mutexState.Int()&mutexLocked, mutexLocked)
}

func TestSetReportInfoNoBlockingSetsWhenObtainingLock(t *testing.T) {
	gatedReportInfo := GatedReportInfo{
		AccountInventoryReports: make(AccountK8SInventoryReports, 1),
	}
	accountName := "testAccount"
	count := 0

	SetReportInfoNoBlocking(accountName, count, reportInfo, &gatedReportInfo)

	assert.Equal(t, reportInfo, gatedReportInfo.AccountInventoryReports[accountName])
	// check mutex is unlocked after operation
	mutexState := reflect.ValueOf(&gatedReportInfo.AccessGate).Elem().FieldByName("w").FieldByName("state")
	assert.Equal(t, mutexState.Int()&mutexLocked, int64(0))
}

func TestSetReportInfoNoBlockingSkipsWhenLockAlreadyTaken(t *testing.T) {
	gatedReportInfo := GatedReportInfo{
		AccountInventoryReports: make(AccountK8SInventoryReports, 1),
	}
	gatedReportInfo.AccessGate.Lock()

	accountName := "testAccount"
	count := 0

	SetReportInfoNoBlocking(accountName, count, reportInfo, &gatedReportInfo)

	assert.NotContains(t, gatedReportInfo.AccountInventoryReports, accountName)
	// check mutex is still locked after operation
	mutexState := reflect.ValueOf(&gatedReportInfo.AccessGate).Elem().FieldByName("w").FieldByName("state")
	assert.Equal(t, mutexState.Int()&mutexLocked, mutexLocked)
}
