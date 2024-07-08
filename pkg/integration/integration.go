package integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	jstime "github.com/anchore/k8s-inventory/internal/time"
)

const MaxAttempts = 2
const IntegrationType = "anchore_k8s_inventory_agent"
const AppVersionLabel = "app.kubernetes.io/version"
const registerAPIPathV2 = "v2/system/integrations/{{id}}/register"

type Namespaces struct {
	Static  []string
	Dynamic []string
}

type Integration struct {
	Id              string             `json:"id,omitempty"`               // uuid that uniquely over, space and time, identifies integration instance
	Type            string             `json:"type,omitempty"`             // type of integration (e.g., 'anchore-k8s-agent')
	Name            string             `json:"name,omitempty"`             // name of the integration instance (e.g., k8s-agent-admin')
	Description     string             `json:"description,omitempty"`      // short description of integration instance
	Version         string             `json:"version,omitempty"`          // version of the integration instance
	State           string             `json:"state,omitempty"`            // state of the integration (Read-only)
	StartedAt       jstime.Datetime    `json:"started_at,omitempty"`       // timestamp when integration instance was started in UTC().Format(time.RFC3339)
	Uptime          jstime.Duration    `json:"uptime,omitempty"`           // running time of integration instance
	DefaultUsername string             `json:"default_username,omitempty"` // user that the integration instance authenticates as during registration
	DefaultAccount  string             `json:"default_account,omitempty"`  // default account that the integration instance authenticates as during registration
	Accounts        []string           `json:"accounts,omitempty"`         // accounts that the integration instance handles
	Namespaces      Namespaces         `json:"namespaces,omitempty"`       // namespaces that the integration instance handles
	Configuration   config.Application `json:"configuration,omitempty"`    // configuration for the integration instance
	ClusterName     string             `json:"cluster_name,omitempty"`     // name of cluster where the integration instance runs
	Namespace       string             `json:"namespace,omitempty"`        // uuid for namespace that the integration instance belongs to
}

func Register(integration *Integration, registrationDetails config.AnchoreInfo) error {
	var err error

	// there should ever only be one re-registration with a new id
	for i := MaxAttempts; i > 0; i-- {
		var newIntegrationId string

		newIntegrationId, err = register(integration, registrationDetails)
		if err == nil {
			log.Infof("Successfully Registered %s agent: %s(%s) with %s", integration.Type, integration.Name,
				integration.Id, registrationDetails.URL)
			return nil
		}
		if newIntegrationId == "" {
			break
		}
		if i > 1 {
			log.Infof("Attempting to re-register agent (id: %s) with new id: %s", integration.Id, newIntegrationId)
			integration.Id = newIntegrationId
		}
	}
	log.Errorf("Failed to register integration agent: %s", err)
	return err
}

func register(integration *Integration, registrationDetails config.AnchoreInfo) (string, error) {
	log.Infof("Registering %s agent: %s(%s) with %s", integration.Type, integration.Name, integration.Id,
		registrationDetails.URL)
	requestBody, err := json.Marshal(integration)
	if err != nil {
		return "", fmt.Errorf("failed to serialize integration registration as JSON: %w", err)
	}
	_, err = anchore.Put(requestBody, integration.Id, registerAPIPathV2, registrationDetails, "integration registration")
	if err != nil {
		return newId(err), err
	}
	return "", nil
}

func newId(putErr error) string {
	var apiClientErr *anchore.ErrAchoreAPIClient

	if errors.As(putErr, &apiClientErr) {
		if *apiClientErr.Body != nil {
			apiError := anchore.AnchoreAPIError{}
			err := json.Unmarshal(*apiClientErr.Body, &apiError)
			if err == nil && apiError.Message == "Re-register with different id" {
				return apiError.Detail["new_integration_id"].(string)
			}
		}
	}
	return ""
}
