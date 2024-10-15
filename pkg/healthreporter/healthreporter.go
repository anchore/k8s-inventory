package healthreporter

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	jstime "github.com/anchore/k8s-inventory/internal/time"
	intg "github.com/anchore/k8s-inventory/pkg/integration"
)

const healthProtocolVersion = 1
const healthDataVersion = 1
const healthDataType = "k8s_inventory_agent"
const HealthReportAPIPathV2 = "v2/system/integrations/{{id}}/health-report"

type HealthReport struct {
	UUID                 string           `json:"uuid,omitempty"`                   // uuid for this health report
	ProtocolVersion      int              `json:"protocol_version,omitempty"`       // protocol version for "common" part of health reporting
	Timestamp            jstime.Datetime  `json:"timestamp,omitempty"`              // timestamp for this health report in UTC().Format(time.RFC3339)
	Uptime               *jstime.Duration `json:"uptime,omitempty"`                 // running time of integration instance
	HealthReportInterval int              `json:"health_report_interval,omitempty"` // time in seconds between health reports
	HealthData           HealthData       `json:"health_data,omitempty"`            // K8s-inventory agent specific health data
}

type HealthData struct {
	Type    string             `json:"type,omitempty"`    // type of health data
	Version int                `json:"version,omitempty"` // format version
	Errors  HealthReportErrors `json:"errors,omitempty"`  // list of errors
	// Anything below this line is specific to k8s-inventory-agent
	AccountK8sInventoryReports AccountK8SInventoryReports `json:"account_k8s_inventory_reports,omitempty"` // latest inventory reports per account
}

type HealthReportErrors []string

// AccountK8SInventoryReports holds per account information about latest inventory reports from the same batch set
type AccountK8SInventoryReports map[string]InventoryReportInfo

type InventoryReportInfo struct {
	ReportTimestamp     string      `json:"report_timestamp"`      // Timestamp for the inventory report that was batched
	Account             string      `json:"account_name"`          // Name of account to which the inventory report belongs
	SentAsUser          string      `json:"sent_as_user"`          // User that the inventory report was sent as
	BatchSize           int         `json:"batch_size"`            // Number of batches that the inventory report was sent in
	LastSuccessfulIndex int         `json:"last_successful_index"` // Index of last successfully sent batch, -1 if none
	HasErrors           bool        `json:"has_errors"`            // HasErrors is true if any of the batches had an error, false otherwise
	Batches             []BatchInfo `json:"batches"`               // Information about each inventory report batch
}

type BatchInfo struct {
	BatchIndex    int             `json:"batch_index,omitempty"`    // Index of this inventory report batch item
	SendTimestamp jstime.Datetime `json:"send_timestamp,omitempty"` // Timestamp when the batch was sent, in UTC().Format(time.RFC3339)
	Error         string          `json:"error,omitempty"`          // Any error this batch encountered when sent
}

// GatedReportInfo The go routine that generates the inventory report must inform the go routine
// that sends health reports about the *latest* sent inventory reports.
// A buffered channel is FIFO so the earliest inserted items are returned first. No new items can
// be added when the buffer is full. This means that the information about the latest sent health
// reports will have to be dropped in such situations. We would rather drop the information about
// the *oldest* sent health reports.
// We therefore use a map (key'ed by account) to store information about the latest sent inventory
// reports This map is shared by the go routine that generates inventory reports and the go
// routine that sends health reports. Access to the map is coordinated by a mutex.
type GatedReportInfo struct {
	AccessGate              sync.RWMutex
	AccountInventoryReports AccountK8SInventoryReports
}

type _NewUUID func() uuid.UUID

type _Now func() time.Time

func GetGatedReportInfo() *GatedReportInfo {
	return &GatedReportInfo{
		AccountInventoryReports: make(AccountK8SInventoryReports),
	}
}

func PeriodicallySendHealthReport(cfg *config.Application, ch intg.Channels, gatedReportInfo *GatedReportInfo) {
	// Wait for registration with Enterprise to be completed
	integration := <-ch.IntegrationObj
	log.Info("Health reporting started")

	ticker := time.NewTicker(time.Duration(cfg.HealthReportIntervalSeconds) * time.Second)

	for {
		log.Infof("Waiting %d seconds to send health report...", cfg.HealthReportIntervalSeconds)

		_, _ = sendHealthReport(cfg, integration, gatedReportInfo, uuid.New, time.Now)
		// log.Debugf("Start new health report: %s", <-ticker.C)
		<-ticker.C
	}
}

func sendHealthReport(cfg *config.Application, integration *intg.Integration, gatedReportInfo *GatedReportInfo, newUUID _NewUUID, _now _Now) (*HealthReport, error) {
	healthReportID := newUUID().String()
	lastReports := GetAccountReportInfoNoBlocking(gatedReportInfo, cfg, _now)

	now := _now().UTC()
	integration.Uptime = &jstime.Duration{Duration: now.Sub(integration.StartedAt.Time)}
	healthReport := HealthReport{
		UUID:            healthReportID,
		ProtocolVersion: healthProtocolVersion,
		Timestamp:       jstime.Datetime{Time: now},
		Uptime:          integration.Uptime,
		HealthData: HealthData{
			Type:                       healthDataType,
			Version:                    healthDataVersion,
			Errors:                     make(HealthReportErrors, 0),
			AccountK8sInventoryReports: lastReports,
		},
		HealthReportInterval: cfg.HealthReportIntervalSeconds,
	}

	log.Infof("Sending health report (uuid:%s) covering %d accounts", healthReport.UUID, len(healthReport.HealthData.AccountK8sInventoryReports))
	requestBody, err := json.Marshal(healthReport)
	if err != nil {
		log.Errorf("failed to serialize integration registration as JSON: %v", err)
		return nil, err
	}
	_, err = anchore.Post(requestBody, integration.UUID, HealthReportAPIPathV2, cfg.AnchoreDetails, "health report")
	if err != nil {
		log.Errorf("Failed to send health report to Anchore: %v", err)
		return nil, err
	}
	return &healthReport, nil
}

func GetAccountReportInfoNoBlocking(gatedReportInfo *GatedReportInfo, cfg *config.Application, _now _Now) AccountK8SInventoryReports {
	locked := gatedReportInfo.AccessGate.TryLock()

	if locked {
		defer gatedReportInfo.AccessGate.Unlock()

		log.Debugf("Removing inventory report info for accounts that are no longer active")
		accountsToRemove := make(map[string]bool)
		now := _now().UTC()
		inactiveAge := 2 * float64(cfg.PollingIntervalSeconds)

		for account, reportInfo := range gatedReportInfo.AccountInventoryReports {
			for _, batchInfo := range reportInfo.Batches {
				log.Debugf("Last inv.report (time:%s, account:%s, batch:%d/%d, sent:%s error:'%s')",
					reportInfo.ReportTimestamp, account, batchInfo.BatchIndex, reportInfo.BatchSize,
					batchInfo.SendTimestamp, batchInfo.Error)
				reportTime, err := time.Parse(time.RFC3339, reportInfo.ReportTimestamp)
				if err != nil {
					log.Errorf("failed to parse report_timestamp: %v", err)
					continue
				}
				if now.Sub(reportTime).Seconds() > inactiveAge {
					accountsToRemove[account] = true
				}
			}
		}

		for accountToRemove := range accountsToRemove {
			log.Debugf("Accounts no longer considered active: %s", accountToRemove)
			delete(gatedReportInfo.AccountInventoryReports, accountToRemove)
		}

		return gatedReportInfo.AccountInventoryReports
	}
	log.Debugf("Unable to obtain mutex lock to get aocount inventory report information. Continuing.")
	return AccountK8SInventoryReports{}
}

func SetReportInfoNoBlocking(accountName string, count int, reportInfo InventoryReportInfo, gatedReportInfo *GatedReportInfo) {
	log.Debugf("Setting report (%s) for account name '%s': %d/%d %s %s", reportInfo.ReportTimestamp, accountName,
		reportInfo.Batches[count].BatchIndex, reportInfo.BatchSize, reportInfo.Batches[count].SendTimestamp,
		reportInfo.Batches[count].Error)
	locked := gatedReportInfo.AccessGate.TryLock()
	if locked {
		defer gatedReportInfo.AccessGate.Unlock()
		gatedReportInfo.AccountInventoryReports[accountName] = reportInfo
	} else {
		// we prioritize no blocking over actually bookkeeping info for every sent inventory report
		log.Debugf("Unable to obtain mutex lock to include inventory report timestamped %s for %s: %d/%d %s in health report. Continuing.",
			reportInfo.ReportTimestamp, accountName, reportInfo.Batches[count].BatchIndex, reportInfo.BatchSize,
			reportInfo.Batches[count].SendTimestamp)
	}
}
