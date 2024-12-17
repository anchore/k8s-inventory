package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/anchore/k8s-inventory/pkg/client"
	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	jstime "github.com/anchore/k8s-inventory/internal/time"
)

var requiredAnchoreVersion, _ = version.NewVersion("5.11")

var inventoryReportingActive = false

const Type = "k8s_inventory_agent"
const RegisterAPIPathV2 = "v2/system/integrations/registration"
const AppVersionLabel = "app.kubernetes.io/version"

type Channels struct {
	IntegrationObj            chan *Integration
	HealthReportingEnabled    chan bool
	InventoryReportingEnabled chan bool
}

// HealthStatus reflects the state of the Integration wrt any errors
// encountered when performing its tasks
type HealthStatus struct {
	State   string `json:"state,omitempty"` // state of the integration HEALTHY or UNHEALTHY
	Reason  string `json:"reason,omitempty"`
	Details any    `json:"details,omitempty"`
}

// LifeCycleStatus reflects the state of the Integration from the perspective of Enterprise
type LifeCycleStatus struct {
	State     string          `json:"state,omitempty"` // lifecycle state REGISTERED, ACTIVE, DEGRADED, DEACTIVATED
	Reason    string          `json:"reason,omitempty"`
	Details   any             `json:"details,omitempty"`
	UpdatedAt jstime.Datetime `json:"updated_at,omitempty"`
}

type Integration struct {
	UUID                   string                 `json:"uuid,omitempty"`                     // uuid provided to this integration instance during
	Type                   string                 `json:"type,omitempty"`                     // type of integration (e.g., 'anchore-k8s-agent')
	Name                   string                 `json:"name,omitempty"`                     // name of the integration instance (e.g., k8s-agent-admin')
	Description            string                 `json:"description,omitempty"`              // short description of integration instance
	Version                string                 `json:"version,omitempty"`                  // version of the integration instance
	ReportedStatus         *HealthStatus          `json:"reported_status,omitempty"`          // health status of the integration (Read-only)
	IntegrationStatus      *LifeCycleStatus       `json:"integration_status,omitempty"`       // lifecycle status of the integration (Read-only)
	StartedAt              jstime.Datetime        `json:"started_at,omitempty"`               // timestamp when integration instance was started in UTC().Format(time.RFC3339)
	LastSeen               *jstime.Datetime       `json:"last_seen,omitempty"`                // timestamp of last received health report from integration instance (Read-only)
	Uptime                 *jstime.Duration       `json:"uptime,omitempty"`                   // running time of integration instance
	Username               string                 `json:"username,omitempty"`                 // user that the integration instance authenticates as during registration
	AccountName            string                 `json:"account_name,omitempty"`             // default account that the integration instance authenticates as during registration
	ExplicitlyAccountBound []string               `json:"explicitly_account_bound,omitempty"` // accounts that the integration instance is explicitly configured to handle
	Accounts               []string               `json:"accounts,omitempty"`                 // names of accounts that the integration instance handled recently
	Namespaces             []string               `json:"namespaces,omitempty"`               // namespaces that the integration instance handles
	Configuration          map[string]interface{} `json:"configuration,omitempty"`            // configuration for the integration instance
	ClusterName            string                 `json:"cluster_name,omitempty"`             // name of cluster where the integration instance runs
	Namespace              string                 `json:"namespace,omitempty"`                // uuid for namespace that the integration instance belongs to
	HealthReportInterval   int                    `json:"health_report_interval,omitempty"`   // time in seconds between health reports
	RegistrationID         string                 `json:"registration_id,omitempty"`          // uuid that integration used during registration
	RegistrationInstanceID string                 `json:"registration_instance_id,omitempty"` // instance id used by the integration during registration
}

type Registration struct {
	RegistrationID         string              `json:"registration_id,omitempty"`          // uyid that identifies integration during registration
	RegistrationInstanceID string              `json:"registration_instance_id,omitempty"` // identifier that make integration instance unique among its replicas during registration
	Type                   string              `json:"type,omitempty"`                     // type of integration (e.g., 'anchore-k8s-agent')
	Name                   string              `json:"name,omitempty"`                     // name of the integration instance (e.g., k8s-agent-admin')
	Description            string              `json:"description,omitempty"`              // short description of integration instance
	Version                string              `json:"version,omitempty"`                  // version of the integration instance
	StartedAt              jstime.Datetime     `json:"started_at,omitempty"`               // timestamp when integration instance was started in UTC().Format(time.RFC3339)
	Uptime                 *jstime.Duration    `json:"uptime,omitempty"`                   // running time of integration instance
	Username               string              `json:"username,omitempty"`                 // user that the integration instance authenticates as during registration
	ExplicitlyAccountBound []string            `json:"explicitly_account_bound,omitempty"` // accounts that the integration instance is explicitly configured to handle
	Namespaces             []string            `json:"namespaces,omitempty"`               // namespaces that the integration instance is explicitly configured to handle
	Configuration          *config.Application `json:"configuration,omitempty"`            // configuration for the integration instance
	ClusterName            string              `json:"cluster_name,omitempty"`             // name of cluster where the integration instance runs
	Namespace              string              `json:"namespace,omitempty"`                // uuid for namespace that the integration instance belongs to
	HealthReportInterval   int                 `json:"health_report_interval,omitempty"`   // time in seconds between health reports
}

type _NewUUID func() uuid.UUID

type _Now func() time.Time

func PerformRegistration(appConfig *config.Application, ch Channels) (*Integration, error) {
	defer closeChannels(ch)

	_, err := awaitVersion(appConfig.AnchoreDetails, ch, -1, 2*time.Second, 1*time.Hour)
	if err != nil {
		return nil, err
	}

	namespace := os.Getenv("POD_NAMESPACE")
	name := os.Getenv("HOSTNAME")

	k8sClient := getK8sClient(appConfig)
	registrationInfo := getRegistrationInfo(appConfig, k8sClient, namespace, name, uuid.New, time.Now)

	// Register this agent with enterprise
	registeredIntegration, err := register(registrationInfo, appConfig.AnchoreDetails, -1,
		2*time.Second, 10*time.Minute, time.Now)
	if err != nil {
		log.Errorf("Unable to register agent: %v", err)
		return nil, err
	}

	enableHealthReporting(ch, registeredIntegration)

	if !inventoryReportingActive {
		enableInventoryReporting(ch)
	}

	return registeredIntegration, nil
}

func awaitVersion(anchoreDetails config.AnchoreInfo, ch Channels, maxRetry int, startBackoff, maxBackoff time.Duration) (*anchore.Version, error) {
	attempt := 0
	for {
		retry := false

		anchoreVersion, err := anchore.GetVersion(anchoreDetails)
		if err == nil {
			ver, vErr := version.NewVersion(anchoreVersion.Service.Version)
			if vErr != nil {
				log.Errorf("Failed to parse received service version: %v. Will try again in %s", vErr, startBackoff)
				retry = true
			} else {
				log.Infof("Successfully determined service version: %s for Enterprise: %s",
					anchoreVersion.Service.Version, anchoreDetails.URL)
				if ver.GreaterThanOrEqual(requiredAnchoreVersion) {
					log.Infof("Proceeding with integration registration since Enterprise v%s supports that", anchoreVersion.Service.Version)
					return anchoreVersion, nil
				}
				if !inventoryReportingActive {
					log.Infof("Proceeding without integration registration and health reporting since Enterprise v%s does not support that",
						anchoreVersion.Service.Version)
					enableInventoryReporting(ch)
				}
				retry = true
			}
		}

		attempt++
		if maxRetry >= 0 && attempt > maxRetry {
			log.Infof("Failed to get Enterprise version after %d attempts", attempt)
			return nil, fmt.Errorf("failed to get Enterprise version after %d attempts", attempt)
		}

		if anchore.ServerIsOffline(err) {
			log.Infof("Anchore is offline. Will try again in %s", startBackoff)
			retry = true
		}

		if retry {
			time.Sleep(startBackoff)
			if startBackoff < maxBackoff {
				startBackoff = min(startBackoff*2, maxBackoff)
			}
			continue
		}

		log.Errorf("Failed to get service version for Enterprise: %s, %v", anchoreDetails.URL, err)
		return nil, err
	}
}

func GetChannels() Channels {
	return Channels{
		IntegrationObj:            make(chan *Integration),
		HealthReportingEnabled:    make(chan bool, 1), // buffered to prevent registration from blocking
		InventoryReportingEnabled: make(chan bool),
	}
}

func closeChannels(ch Channels) {
	close(ch.IntegrationObj)
	close(ch.HealthReportingEnabled)
	close(ch.InventoryReportingEnabled)
}

func enableHealthReporting(ch Channels, integration *Integration) {
	log.Info("Activating health reporting")
	// signal health reporting to start by providing it with the integration
	ch.IntegrationObj <- integration
	// signal inventory reporting to populate health report info when generating inventory reports
	ch.HealthReportingEnabled <- true
}

func enableInventoryReporting(ch Channels) {
	inventoryReportingActive = true
	log.Info("Activating inventory reporting")
	// signal inventory reporting to start
	ch.InventoryReportingEnabled <- true
}

func getK8sClient(appConfig *config.Application) *client.Client {
	kubeconfig, err := client.GetKubeConfig(appConfig)
	if err != nil {
		log.Errorf("Failed to get Kubernetes config: %v", err)
		return nil
	}

	clientset, err := client.GetClientSet(kubeconfig)
	if err != nil {
		log.Errorf("Failed to get k8s client set: %v", err)
		return nil
	}

	return &client.Client{
		Clientset: clientset,
	}
}

func register(registrationInfo *Registration, anchoreDetails config.AnchoreInfo, maxRetry int,
	startBackoff, maxBackoff time.Duration, now _Now) (*Integration, error) {
	var err error

	attempt := 0
	for {
		var registeredIntegration *Integration

		registeredIntegration, err = doRegister(registrationInfo, anchoreDetails, now)
		if err == nil {
			log.Infof("Successfully registered %s agent: %s (registration_id:%s / registration_instance_id:%s) with %s",
				registrationInfo.Type, registrationInfo.Name, registrationInfo.RegistrationID,
				registrationInfo.RegistrationInstanceID, anchoreDetails.URL)
			log.Infof("This agent's integration uuid is %s", registeredIntegration.UUID)
			return registeredIntegration, nil
		}

		attempt++
		if maxRetry >= 0 && attempt > maxRetry {
			log.Errorf("Failed to register agent (registration_id:%s / registration_instance_id:%s) after %d attempts",
				registrationInfo.RegistrationID, registrationInfo.RegistrationInstanceID, attempt)
			return nil, fmt.Errorf("failed to register after %d attempts", attempt)
		}

		if anchore.ServerIsOffline(err) {
			log.Infof("Anchore is offline. Will try again in %s", startBackoff)
			time.Sleep(startBackoff)
			if startBackoff < maxBackoff {
				startBackoff = min(startBackoff*2, maxBackoff)
			}
			continue
		}

		if anchore.UserLacksAPIPrivileges(err) {
			log.Errorf("Specified user lacks required privileges to register and send health reports %v", err)
			return nil, err
		}

		if anchore.IncorrectCredentials(err) {
			log.Errorf("Failed to register due to invalid credentials (wrong username or password")
			return nil, err
		}

		log.Errorf("Failed to register integration agent (registration_id:%s / regitration_instance_id:%s): %v",
			registrationInfo.RegistrationID, registrationInfo.RegistrationInstanceID, err)
		return nil, err
	}
}

func doRegister(registrationInfo *Registration, anchoreDetails config.AnchoreInfo, now _Now) (*Integration, error) {
	log.Infof("Registering %s agent: %s (registration_id:%s / regitration_instance_id:%s) with %s",
		registrationInfo.Type, registrationInfo.Name, registrationInfo.RegistrationID,
		registrationInfo.RegistrationInstanceID, anchoreDetails.URL)

	registrationInfo.Uptime = &jstime.Duration{Duration: now().UTC().Sub(registrationInfo.StartedAt.Time)}
	requestBody, err := json.Marshal(registrationInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize integration registration as JSON: %w", err)
	}
	responseBody, err := anchore.Post(requestBody, "", RegisterAPIPathV2, anchoreDetails, "integration registration")
	if err != nil {
		return nil, err
	}
	registeredIntegration := Integration{}
	err = json.Unmarshal(*responseBody, &registeredIntegration)
	return &registeredIntegration, err
}

func getRegistrationInfo(appConfig *config.Application, k8sClient *client.Client,
	namespace string, name string, newUUID _NewUUID, now _Now) *Registration {
	var registrationID, registrationInstanceID, instanceName, appVersion, description string

	log.Debugf("Attempting to determine values from K8s Deployment for Pod: %s in Namespace: %s",
		name, namespace)
	registrationID, instanceName, appVersion = getInstanceDataFromK8s(k8sClient, namespace, name)

	if appConfig.Registration.RegistrationID != "" {
		log.Debugf("Using registration_id specified in config: %s", appConfig.Registration.RegistrationID)
		registrationID = appConfig.Registration.RegistrationID
	}

	if registrationID == "" {
		log.Debugf("The registration_id value is not valid. Generating UUIDv4 to use as registration_id")
		registrationID = newUUID().String()
	}

	if name != "" {
		log.Debugf("Using registration_instance_id: %s", name)
		registrationInstanceID = name
	} else {
		log.Debugf("Generating UUIDv4 to use as registration_instance_id")
		registrationInstanceID = newUUID().String()
	}

	if appConfig.Registration.IntegrationName != "" {
		log.Debugf("Using name for integration specified in config: %s", appConfig.Registration.IntegrationName)
		instanceName = appConfig.Registration.IntegrationName
	}

	if appConfig.Registration.IntegrationDescription != "" {
		log.Debugf("Using description for integration specified in config: %s",
			appConfig.Registration.IntegrationDescription)
		description = appConfig.Registration.IntegrationDescription
	}

	log.Debugf("Integration registration_id: %s, registration_instance_id: %s, name: %s, description: %s",
		registrationID, registrationInstanceID, instanceName, description)

	explicitlyAccountBound, namespaces := getAccountsAndNamespacesForAgent(appConfig)

	instance := Registration{
		RegistrationID:         registrationID,
		RegistrationInstanceID: registrationInstanceID,
		Type:                   Type,
		Name:                   instanceName,
		Description:            description,
		Version:                appVersion,
		StartedAt:              jstime.Datetime{Time: now().UTC()},
		Uptime:                 new(jstime.Duration),
		Username:               appConfig.AnchoreDetails.User,
		ExplicitlyAccountBound: explicitlyAccountBound,
		Namespaces:             namespaces,
		Configuration:          nil,
		ClusterName:            appConfig.KubeConfig.Cluster,
		Namespace:              namespace,
		HealthReportInterval:   appConfig.HealthReportIntervalSeconds,
	}
	return &instance
}

func getInstanceDataFromK8s(k8sClient *client.Client, namespace string, podName string) (string, string, string) {
	if k8sClient == nil {
		log.Errorf("Kubernetes client not initialized. Unable to interact with K8s cluster.")
		return "", "", ""
	}
	opts := metav1.GetOptions{}
	pod, err := k8sClient.Clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, opts)
	if err != nil {
		log.Errorf("failed to get pod: %v", err)
		return "", "", ""
	}
	replicaSetName := pod.ObjectMeta.OwnerReferences[0].Name
	replicaSet, err := k8sClient.Clientset.AppsV1().ReplicaSets(namespace).Get(context.Background(), replicaSetName, opts)
	if err != nil {
		log.Errorf("failed to get replica set: %v", err)
		return "", "", ""
	}
	deploymentName := replicaSet.ObjectMeta.OwnerReferences[0].Name
	deployment, err := k8sClient.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, opts)
	if err != nil {
		log.Errorf("failed to get deployment: %v", err)
		return "", "", ""
	}

	appVersion := deployment.Labels[AppVersionLabel]
	registrationID := fmt.Sprint("", deployment.ObjectMeta.UID)
	instanceName := deploymentName
	log.Debugf("Determined integration values for agent from K8s, registration_id: %s, instance_name: %s, appVersion: %s",
		registrationID, instanceName, appVersion)
	return registrationID, instanceName, appVersion
}

func getAccountsAndNamespacesForAgent(appConfig *config.Application) ([]string, []string) {
	accountSet := make(map[string]bool)
	namespaceSet := make(map[string]bool)

	// pick up accounts that are explicitly listed in the config
	for account, accountRouteDetails := range appConfig.AccountRoutes {
		accountSet[account] = true
		for _, namespace := range accountRouteDetails.Namespaces {
			namespaceSet[namespace] = true
		}
	}
	accounts := make([]string, 0, len(accountSet))
	for account := range accountSet {
		accounts = append(accounts, account)
	}

	namespaces := make([]string, 0, len(namespaceSet))
	for namespace := range namespaceSet {
		namespaces = append(namespaces, namespace)
	}

	return accounts, namespaces
}
