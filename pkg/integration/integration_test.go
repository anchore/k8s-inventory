package integration

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	jstime "github.com/anchore/k8s-inventory/internal/time"
	"github.com/anchore/k8s-inventory/pkg/client"
	"github.com/google/uuid"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	versionMap511 = map[string]interface{}{
		"service": map[string]interface{}{
			"version": "5.11.0",
		},
		"api": map[string]interface{}{
			"version": "2",
		},
		"db": map[string]interface{}{
			"schema_version": "5110",
		},
	}

	versionMap510 = map[string]interface{}{
		"service": map[string]interface{}{
			"version": "5.10.0",
		},
		"api": map[string]interface{}{
			"version": "2",
		},
		"db": map[string]interface{}{
			"schema_version": "510",
		},
	}

	versionObj511 = anchore.Version{
		API: struct {
			Version string `json:"version"`
		}(struct {
			Version string
		}{"2"}),
		DB: struct {
			SchemaVersion string `json:"schema_version"`
		}(struct {
			SchemaVersion string
		}{"5110"}),
		Service: struct {
			Version string `json:"version"`
		}(struct {
			Version string
		}{"5.11.0"}),
	}

	now        = time.Date(2024, 10, 4, 10, 11, 12, 0, time.Local)
	timestamps = []time.Time{now.Add(time.Second * 2), now.Add(time.Second * 4), now.Add(time.Second * 6)}

	integration = map[string]interface{}{
		"uuid":                     "000d1e60-cb05-4cce-8d1e-60cb052cce1f",
		"type":                     "k8s_inventory_agent",
		"name":                     "k8s-inv-agent",
		"description":              "k8s-agent with health reporting",
		"version":                  "2.0",
		"reported_status":          map[string]interface{}{"state": "HEALTHY"},
		"integration_status":       map[string]interface{}{"state": "REGISTERED"},
		"started_at":               now.UTC().Format(time.RFC3339),
		"last_seen":                nil,
		"uptime":                   2,
		"username":                 "account0User",
		"account_name":             "account0",
		"explicitly_account_bound": []interface{}{},
		"accounts":                 []interface{}{},
		"namespaces":               []interface{}{},
		"cluster_name":             "Docker-Desktop",
		"namespace":                "default",
		"health_report_interval":   60,
		"registration_id":          "de2c3c58-4c20-4d87-ac3c-584c201d875a",
		"registration_instance_id": "45743315",
	}

	integrationInstance = Integration{
		UUID:                   "000d1e60-cb05-4cce-8d1e-60cb052cce1f",
		Type:                   "k8s_inventory_agent",
		Name:                   "k8s-inv-agent",
		Description:            "k8s-agent with health reporting",
		Version:                "2.0",
		ReportedStatus:         nil,
		IntegrationStatus:      nil,
		StartedAt:              jstime.Datetime{Time: now.UTC()},
		LastSeen:               nil,
		Uptime:                 nil,
		Username:               "account0User",
		AccountName:            "account0",
		ExplicitlyAccountBound: []string{},
		Accounts:               []string{},
		Namespaces:             []string{},
		ClusterName:            "Docker-Desktop",
		Namespace:              "default",
		HealthReportInterval:   60,
		RegistrationID:         "de2c3c58-4c20-4d87-ac3c-584c201d875a",
		RegistrationInstanceID: "45743315",
	}

	pod = v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			UID:  "test-pod-uid",
			Annotations: map[string]string{
				"test-annotation": "test-value",
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
				"app.kubernetes.io/name":       "k8s-inventory",
				"app.kubernetes.io/version":    "1.7.0",
				"helm.sh/chart":                "k8s-inventory-0.5.0",
			},
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-replicaset",
					UID:        "test-replicaset-uid",
				},
			},
		},
	}

	replicaSet = appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-replicaset",
			UID:  "test-replicaset-uid",
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "my-k8s-inventory-release",
				"meta.helm.sh/release-namespace": "test-namespace",
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
				"app.kubernetes.io/name":       "k8s-inventory",
				"app.kubernetes.io/version":    "1.7.0",
				"helm.sh/chart":                "k8s-inventory-0.5.0",
			},
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Name:       "test-deployment-k8s-inventory",
					Kind:       "Deployment",
					UID:        "test-deployment-uid",
				},
			},
		},
	}

	deployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment-k8s-inventory",
			UID:  "test-deployment-uid",
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "my-k8s-inventory-release",
				"meta.helm.sh/release-namespace": "test-namespace",
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
				"app.kubernetes.io/name":       "k8s-inventory",
				"app.kubernetes.io/version":    "1.7.0",
				"helm.sh/chart":                "k8s-inventory-0.5.0",
			},
			Namespace: "test-namespace",
		},
	}
)

func TestAwaitVersion(t *testing.T) {
	defer gock.Off()

	anchoreDetails := config.AnchoreInfo{
		URL:  "https://ancho.re",
		User: "admin",
	}
	type want struct {
		version *anchore.Version
		err     error
	}
	tests := []struct {
		name string
		want want
	}{
		{
			name: "successful await version, 5.11.0",
			want: want{
				version: &versionObj511,
				err:     nil,
			},
		},
		{
			name: "successful await version, <5.11.0",
			want: want{
				version: nil,
				err:     fmt.Errorf("failed to get Enterprise version after 2 attempts"),
			},
		},
		{
			name: "enterprise offline on first attempt",
			want: want{
				version: &versionObj511,
				err:     nil,
			},
		},
		{
			name: "other error",
			want: want{
				version: nil,
				err: &anchore.APIClientError{
					HTTPStatusCode: http.StatusBadRequest,
					Message:        "400 Bad Request response from Anchore (during version get)",
					Path:           "/version",
					Method:         "GET",
				},
			},
		},
	}
	for _, tt := range tests {
		gock.Flush()

		ch := GetChannels()
		valueSet := false
		var inventoryReportingEnabledValue bool

		switch tt.name {
		case "successful await version, 5.11.0":
			gock.New("https://ancho.re").
				Get("/version").
				Reply(200).
				JSON(versionMap511)
		case "successful await version, <5.11.0":
			gock.New("https://ancho.re").
				Get("/version").
				Persist().
				Reply(200).
				JSON(versionMap510)
			go func() {
				var setBeforeClosed bool
				inventoryReportingEnabledValue, setBeforeClosed = <-ch.InventoryReportingEnabled
				valueSet = true
				assert.True(t, setBeforeClosed)
			}()
		case "enterprise offline on first attempt":
			gock.New("https://ancho.re").
				Get("/version").
				ReplyError(&url.Error{
					Op:  "Post",
					URL: "https://ancho.re/version",
					Err: &net.OpError{
						Op:     "dial",
						Net:    "tcp",
						Source: nil,
						Addr: &net.TCPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: 8228,
							Zone: "",
						},
						Err: &os.SyscallError{
							Syscall: "connect",
							Err:     syscall.ECONNREFUSED,
						},
					},
				})
			gock.New("https://ancho.re").
				Get("/version").
				Reply(200).
				JSON(versionMap511)
		case "other error":
			gock.New("https://ancho.re").
				Get("/version").
				Reply(http.StatusBadRequest)
		}
		t.Run(tt.name, func(t *testing.T) {
			version, err := awaitVersion(anchoreDetails, ch, 1, 500*time.Millisecond, 10*time.Minute)
			if tt.want.err != nil {
				assert.Error(t, err)
				assert.Equal(t, err, tt.want.err)
				if tt.name == "successful await version, <5.11.0" {
					// this is not a perfect way of testing pre 5.11 release case but better than nothing
					assert.True(t, valueSet)
					assert.True(t, inventoryReportingEnabledValue)
				}
				assert.Nil(t, version)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.version, version)
			}
		})
	}
}

func TestCloseChannels(t *testing.T) {
	ch := GetChannels()
	closeChannels(ch)
	result1, isNotClosed := <-ch.IntegrationObj
	assert.Nil(t, result1)
	assert.False(t, isNotClosed)
	result2, isNotClosed := <-ch.HealthReportingEnabled
	assert.False(t, result2)
	assert.False(t, isNotClosed)
	result3, isNotClosed := <-ch.InventoryReportingEnabled
	assert.False(t, result3)
	assert.False(t, isNotClosed)
}

func TestEnableHealthReporting(t *testing.T) {
	ch := GetChannels()
	integrationInstance := &Integration{
		UUID: "some uuid",
	}
	go enableHealthReporting(ch, integrationInstance)
	result1 := <-ch.IntegrationObj
	result2 := <-ch.HealthReportingEnabled
	assert.Equal(t, integrationInstance, result1)
	assert.True(t, result2)
}

func TestEnableInventoryReporting(t *testing.T) {
	ch := GetChannels()
	go enableInventoryReporting(ch)
	result := <-ch.InventoryReportingEnabled
	assert.True(t, result)
}

func TestRegister(t *testing.T) {
	defer gock.Off()

	type args struct {
		uptime float64
	}
	type want struct {
		integration       Integration
		ReportedStatus    HealthStatus
		IntegrationStatus LifeCycleStatus
		Uptime            jstime.Duration
		err               error
		nilIntegration    bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "successful registration on first attempt",
			args: args{
				uptime: 2.0,
			},
			want: want{
				integration: integrationInstance,
				ReportedStatus: HealthStatus{
					State: "HEALTHY",
				},
				IntegrationStatus: LifeCycleStatus{
					State: "REGISTERED",
				},
				Uptime: jstime.Duration{Duration: time.Second * 2},
				err:    nil,
			},
		},
		{
			name: "enterprise offline on first attempt",
			args: args{
				uptime: 4.0,
			},
			want: want{
				integration: integrationInstance,
				ReportedStatus: HealthStatus{
					State: "HEALTHY",
				},
				IntegrationStatus: LifeCycleStatus{
					State: "REGISTERED",
				},
				Uptime: jstime.Duration{Duration: time.Second * 4},
				err:    nil,
			},
		},
		{
			name: "abort registration on max attempts",
			args: args{
				uptime: 4.0,
			},
			want: want{
				integration:       Integration{},
				ReportedStatus:    HealthStatus{},
				IntegrationStatus: LifeCycleStatus{},
				Uptime:            jstime.Duration{Duration: time.Second * 4},
				err:               fmt.Errorf("failed to register after %d attempts", 2),
			},
		},
		{
			name: "user lacks api privileges",
			args: args{
				uptime: 2.0,
			},
			want: want{
				integration:       Integration{},
				ReportedStatus:    HealthStatus{},
				IntegrationStatus: LifeCycleStatus{},
				Uptime:            jstime.Duration{Duration: time.Second * 2},
				err: &anchore.APIClientError{
					HTTPStatusCode: http.StatusForbidden,
					Message:        "403 Forbidden response from Anchore (during integration registration)",
					Path:           "/v2/system/integrations/registration",
					Method:         "POST",
					APIErrorDetails: &anchore.APIErrorDetails{
						Message:  "Not authorized. Requires permissions: domain=account0 action=registerIntegration target=",
						Detail:   map[string]interface{}{},
						HTTPCode: http.StatusForbidden,
					},
				},
			},
		},
		{
			name: "wrong user credentials",
			args: args{
				uptime: 2.0,
			},
			want: want{
				integration:       Integration{},
				ReportedStatus:    HealthStatus{},
				IntegrationStatus: LifeCycleStatus{},
				Uptime:            jstime.Duration{Duration: time.Second * 2},
				err: &anchore.APIClientError{
					HTTPStatusCode: http.StatusUnauthorized,
					Message:        "401 Unauthorized response from Anchore (during integration registration)",
					Path:           "/v2/system/integrations/registration",
					Method:         "POST",
				},
			},
		},
		{
			name: "other error",
			args: args{
				uptime: 2.0,
			},
			want: want{
				integration:       Integration{},
				ReportedStatus:    HealthStatus{},
				IntegrationStatus: LifeCycleStatus{},
				Uptime:            jstime.Duration{Duration: time.Second * 2},
				err: &anchore.APIClientError{
					HTTPStatusCode: http.StatusBadRequest,
					Message:        "400 Bad Request response from Anchore (during integration registration)",
					Path:           "/v2/system/integrations/registration",
					Method:         "POST",
				},
			},
		},
	}
	for _, tt := range tests {
		gock.Flush()

		anchoreDetails := config.AnchoreInfo{
			URL:  "https://ancho.re",
			User: "admin",
		}
		registrationInfo := &Registration{
			RegistrationID:         "test-registration-id",
			RegistrationInstanceID: "1111223344",
			Type:                   Type,
			Name:                   "test k8s inventory",
			Description:            "Description from config",
			Version:                "",
			StartedAt:              jstime.Datetime{Time: now.UTC()},
			Uptime:                 new(jstime.Duration),
			Username:               "admin",
			ExplicitlyAccountBound: []string{"account3"},
			Namespaces:             []string{"ns3"},
			Configuration:          nil,
			ClusterName:            "k8s-cluster1",
			Namespace:              "test-namespace",
			HealthReportInterval:   60,
		}
		integration["uptime"] = tt.args.uptime

		j := 0
		nowMock := func() time.Time {
			timestamp := timestamps[j]
			j++
			return timestamp
		}

		switch tt.name {
		case "successful registration on first attempt":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(200).
				JSON(integration)
		case "enterprise offline on first attempt":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				ReplyError(&url.Error{
					Op:  "Post",
					URL: "http://127.0.0.1:8228/v2/system/integrations/registration",
					Err: &net.OpError{
						Op:     "dial",
						Net:    "tcp",
						Source: nil,
						Addr: &net.TCPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: 8228,
							Zone: "",
						},
						Err: &os.SyscallError{
							Syscall: "connect",
							Err:     syscall.ECONNREFUSED,
						},
					},
				})
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(200).
				JSON(integration)
		case "abort registration on max attempts":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Persist().
				ReplyError(&url.Error{
					Op:  "Post",
					URL: "http://127.0.0.1:8228/v2/system/integrations/registration",
					Err: &net.OpError{
						Op:     "dial",
						Net:    "tcp",
						Source: nil,
						Addr: &net.TCPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: 8228,
							Zone: "",
						},
						Err: &os.SyscallError{
							Syscall: "connect",
							Err:     syscall.ECONNREFUSED,
						},
					},
				})
		case "user lacks api privileges":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(http.StatusForbidden).
				JSON(map[string]interface{}{
					"message":  "Not authorized. Requires permissions: domain=account0 action=registerIntegration target=",
					"detail":   map[string]interface{}{},
					"httpcode": http.StatusForbidden,
				})
		case "wrong user credentials":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(http.StatusUnauthorized)
		case "other error":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(http.StatusBadRequest)
		}
		t.Run(tt.name, func(t *testing.T) {
			registeredIntegration, err := register(registrationInfo, anchoreDetails, 1,
				500*time.Millisecond, 10*time.Minute, nowMock)
			if tt.want.err != nil {
				assert.Error(t, err)
				assert.Equal(t, err, tt.want.err)
				assert.Nil(t, registeredIntegration)
			} else {
				assert.NoError(t, err)
				if tt.want.nilIntegration {
					assert.Nil(t, registeredIntegration)
				} else {
					integrationStatus := registeredIntegration.IntegrationStatus
					registeredIntegration.IntegrationStatus = nil
					reportedStatus := registeredIntegration.ReportedStatus
					registeredIntegration.ReportedStatus = nil
					uptime := registeredIntegration.Uptime
					registeredIntegration.Uptime = nil
					assert.Equal(t, tt.want.IntegrationStatus, *integrationStatus)
					assert.Equal(t, tt.want.ReportedStatus, *reportedStatus)
					assert.Equal(t, tt.want.Uptime, *uptime)
					assert.Equal(t, tt.want.integration, *registeredIntegration)
				}
				assert.Equal(t, tt.want.Uptime, *registrationInfo.Uptime)
			}
		})
	}
}

func TestGetRegistrationInfo(t *testing.T) {
	uuids := []uuid.UUID{uuid.New(), uuid.New()}
	timestamps := []time.Time{time.Now()}

	type args struct {
		config       *config.Application
		c            *client.Client
		namespace    string
		name         string
		replicaCount int32
	}
	tests := []struct {
		name string
		args args
		want *Registration
	}{
		{
			name: "Generates UUIDs for registration-id and registration-instance-id",
			args: args{
				config: &config.Application{
					AnchoreDetails: config.AnchoreInfo{
						User: "admin",
					},
					AccountRoutes: config.AccountRoutes{
						"account3": config.AccountRouteDetails{
							Namespaces: []string{"ns3"},
						},
					},
					KubeConfig: config.KubeConf{
						Cluster: "k8s-cluster1",
					},
					HealthReportIntervalSeconds: 60,
				},
				c:            nil,
				namespace:    "test-namespace",
				name:         "",
				replicaCount: 1,
			},
			want: &Registration{
				RegistrationID:         uuids[0].String(),
				RegistrationInstanceID: uuids[1].String(),
				Type:                   Type,
				Name:                   "",
				Description:            "",
				Version:                "dev",
				StartedAt:              jstime.Datetime{Time: timestamps[0].UTC()},
				Uptime:                 new(jstime.Duration),
				Username:               "admin",
				ExplicitlyAccountBound: []string{"account3"},
				Namespaces:             []string{"ns3"},
				Configuration:          nil,
				ClusterName:            "k8s-cluster1",
				Namespace:              "test-namespace",
				HealthReportInterval:   60,
			},
		},
		{
			name: "Values from anchore-registration in app config",
			args: args{
				config: &config.Application{
					AnchoreDetails: config.AnchoreInfo{
						User: "admin",
					},
					AccountRoutes: config.AccountRoutes{
						"account3": config.AccountRouteDetails{
							Namespaces: []string{"ns3"},
						},
					},
					KubeConfig: config.KubeConf{
						Cluster: "k8s-cluster1",
					},
					Registration: config.RegistrationOptions{
						RegistrationID:         "test-registration-id",
						IntegrationName:        "test k8s inventory",
						IntegrationDescription: "Description from config",
					},
					HealthReportIntervalSeconds: 60,
				},
				c:            nil,
				namespace:    "test-namespace",
				name:         "1111223344",
				replicaCount: 0,
			},
			want: &Registration{
				RegistrationID:         "test-registration-id",
				RegistrationInstanceID: "1111223344",
				Type:                   Type,
				Name:                   "test k8s inventory",
				Description:            "Description from config",
				Version:                "dev",
				StartedAt:              jstime.Datetime{Time: timestamps[0].UTC()},
				Uptime:                 new(jstime.Duration),
				Username:               "admin",
				ExplicitlyAccountBound: []string{"account3"},
				Namespaces:             []string{"ns3"},
				Configuration:          nil,
				ClusterName:            "k8s-cluster1",
				Namespace:              "test-namespace",
				HealthReportInterval:   60,
			},
		},
		{
			name: "Values from k8s",
			args: args{
				config: &config.Application{
					AnchoreDetails: config.AnchoreInfo{
						User: "admin",
					},
					AccountRoutes: config.AccountRoutes{
						"account3": config.AccountRouteDetails{
							Namespaces: []string{"ns3"},
						},
					},
					KubeConfig: config.KubeConf{
						Cluster: "k8s-cluster1",
					},
					Registration:                config.RegistrationOptions{},
					HealthReportIntervalSeconds: 60,
				},
				c: &client.Client{
					Clientset: fake.NewSimpleClientset(&pod, &replicaSet, &deployment),
				},
				namespace:    "test-namespace",
				name:         "test-pod",
				replicaCount: 2,
			},
			want: &Registration{
				RegistrationID:         "test-deployment-uid",
				RegistrationInstanceID: "test-pod",
				Type:                   Type,
				Name:                   "test-deployment-k8s-inventory",
				Description:            "",
				Version:                "dev",
				StartedAt:              jstime.Datetime{Time: timestamps[0].UTC()},
				Uptime:                 new(jstime.Duration),
				Username:               "admin",
				ExplicitlyAccountBound: []string{"account3"},
				Namespaces:             []string{"ns3"},
				Configuration:          nil,
				ClusterName:            "k8s-cluster1",
				Namespace:              "test-namespace",
				HealthReportInterval:   60,
			},
		},
		{
			name: "Values from k8s single replica",
			args: args{
				config: &config.Application{
					AnchoreDetails: config.AnchoreInfo{
						User: "admin",
					},
					AccountRoutes: config.AccountRoutes{
						"account3": config.AccountRouteDetails{
							Namespaces: []string{"ns3"},
						},
					},
					KubeConfig: config.KubeConf{
						Cluster: "k8s-cluster1",
					},
					Registration:                config.RegistrationOptions{},
					HealthReportIntervalSeconds: 60,
				},
				c: &client.Client{
					Clientset: fake.NewSimpleClientset(&pod, &replicaSet, &deployment),
				},
				namespace:    "test-namespace",
				name:         "test-pod",
				replicaCount: 1,
			},
			want: &Registration{
				RegistrationID:         "test-deployment-uid",
				RegistrationInstanceID: deployment.ObjectMeta.Name,
				Type:                   Type,
				Name:                   "test-deployment-k8s-inventory",
				Description:            "",
				Version:                "dev",
				StartedAt:              jstime.Datetime{Time: timestamps[0].UTC()},
				Uptime:                 new(jstime.Duration),
				Username:               "admin",
				ExplicitlyAccountBound: []string{"account3"},
				Namespaces:             []string{"ns3"},
				Configuration:          nil,
				ClusterName:            "k8s-cluster1",
				Namespace:              "test-namespace",
				HealthReportInterval:   60,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0
			NewUUIDMock := func() uuid.UUID {
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
			result := getRegistrationInfo(tt.args.config, tt.args.c, tt.args.namespace,
				tt.args.name, tt.args.replicaCount, NewUUIDMock, nowMock)
			assert.NotNil(t, result)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetInstanceDataFromK8s(t *testing.T) {
	type args struct {
		c         *client.Client
		namespace string
		podName   string
	}
	type want struct {
		registrationID string
		instanceName   string
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "successful get instance data from k8s",
			args: args{
				c: &client.Client{
					Clientset: fake.NewSimpleClientset(&pod, &replicaSet, &deployment),
				},
				namespace: "test-namespace",
				podName:   "test-pod",
			},
			want: want{
				registrationID: "test-deployment-uid",
				instanceName:   "k8s-inventory",
			},
		},
		{
			name: "nil client",
			args: args{
				c:         nil,
				namespace: "test-namespace",
				podName:   "test-pod",
			},
			want: want{
				registrationID: "",
				instanceName:   "",
			},
		},
		{
			name: "no pod",
			args: args{
				c: &client.Client{
					Clientset: fake.NewSimpleClientset(),
				},
				namespace: "test-namespace",
				podName:   "test-pod",
			},
			want: want{
				registrationID: "",
				instanceName:   "",
			},
		},
		{
			name: "no replicaSet",
			args: args{
				c: &client.Client{
					Clientset: fake.NewSimpleClientset(&pod),
				},
				namespace: "test-namespace",
				podName:   "test-pod",
			},
			want: want{
				registrationID: "",
				instanceName:   "",
			},
		},
		{
			name: "no deployment",
			args: args{
				c: &client.Client{
					Clientset: fake.NewSimpleClientset(&pod, &replicaSet),
				},
				namespace: "test-namespace",
				podName:   "test-pod",
			},
			want: want{
				registrationID: "",
				instanceName:   "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultRegID, resultInstName := getInstanceDataFromK8s(tt.args.c, tt.args.namespace, tt.args.podName)
			assert.Equal(t, tt.want.registrationID, resultRegID)
			assert.NotNil(t, tt.want.instanceName, resultInstName)
		})
	}
}

func TestGetAccountsAndNamespacesForAgent(t *testing.T) {
	type args struct {
		config *config.Application
	}
	type want struct {
		accountNames []string
		namespaces   []string
	}
	tests := []struct {
		name string
		args
		want want
	}{
		{
			name: "empty account routes",
			args: args{
				config: &config.Application{},
			},
			want: want{
				accountNames: []string{},
				namespaces:   []string{},
			},
		},
		{
			name: "populated account routes",
			args: args{
				config: &config.Application{
					AccountRoutes: config.AccountRoutes{
						"account1": config.AccountRouteDetails{
							Namespaces: []string{"ns1", "ns2"},
						},
						"account2": config.AccountRouteDetails{},
						"account3": config.AccountRouteDetails{
							Namespaces: []string{"ns3"},
						},
					},
				},
			},
			want: want{
				accountNames: []string{"account1", "account2", "account3"},
				namespaces:   []string{"ns1", "ns2", "ns3"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultAccountNames, resultNamespaces := getAccountsAndNamespacesForAgent(tt.args.config)
			slices.Sort(resultAccountNames)
			assert.Equal(t, tt.want.accountNames, resultAccountNames)
			slices.Sort(resultNamespaces)
			assert.Equal(t, tt.want.namespaces, resultNamespaces)
		})
	}
}
