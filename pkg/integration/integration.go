package integration

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	jstime "github.com/anchore/k8s-inventory/internal/time"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/h2non/gock"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const IntegrationType = "anchore-k8s-inventory"
const AppVersionLabel = "app.kubernetes.io/version"
const registerAPIPathV2 = "v2/system/integrations/{{id}}/register"

func ErrAnchoreEndpointDoesNotExist(path string) error {
	return fmt.Errorf("api endpoint does not exist: %s", path)
}

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
	log.Infof("Registering %s agent: %s(%s) with %s", integration.Type, integration.Name, integration.Id,
		registrationDetails.URL)
	requestBody, err := json.Marshal(integration)
	if err != nil {
		return fmt.Errorf("failed to serialize integration registration as JSON: %w", err)
	}
	err = Put(requestBody, integration.Id, registerAPIPathV2, registrationDetails, "integration registration")
	if err != nil {
		log.Errorf("Failed to register integration agent: %s", err)
		return err
	}
	log.Infof("Successfully Registered %s agent: %s(%s) with %s", integration.Type, integration.Name, integration.Id,
		registrationDetails.URL)
	return nil
}

func Put(requestBody []byte, id string, path string, anchoreDetails config.AnchoreInfo, operation string) error {
	defer tracker.TrackFunctionTime(time.Now(), fmt.Sprintf("Sent %s request to Anchore", operation))

	log.Debugf("Performing %s to Anchore using endpoint: %s", operation, path)

	client, err := getClient(anchoreDetails)
	if err != nil {
		return err
	}

	anchoreURL, err := getURL(anchoreDetails, path, id)
	if err != nil {
		return err
	}

	request, err := getPutRequest(anchoreDetails, anchoreURL, requestBody, operation)
	if err != nil {
		return err
	}

	return doPut(client, request, operation, path)
}

func getClient(anchoreDetails config.AnchoreInfo) (*http.Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	} // #nosec G402
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}
	gock.InterceptClient(client) // Required to use gock for testing custom client

	return client, nil
}

func getURL(anchoreDetails config.AnchoreInfo, path string, id string) (string, error) {

	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return "", fmt.Errorf("failed to build path (%s) url: %w", path, err)
	}

	anchoreURL.Path += strings.Replace(path, "{{id}}", id, 1)
	return anchoreURL.String(), nil
}

func getPutRequest(anchoreDetails config.AnchoreInfo, endpointURL string, reqBody []byte, operation string) (*http.Request, error) {
	request, err := http.NewRequest("PUT", endpointURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare %s request to Anchore: %w", operation, err)
	}

	request.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-anchore-account", anchoreDetails.Account)
	return request, nil
}

func doPut(client *http.Client, request *http.Request, path string, operation string) error {
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send %s to Anchore: %w", operation, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return fmt.Errorf("failed to perform %s to Anchore: %+v", operation, resp)
	case resp.StatusCode == 403:
		log.Debug("Forbidden response (403) from Anchore (during %s)", operation)
		return fmt.Errorf("user account not found")
	case resp.StatusCode == 404:
		log.Debug("Forbidden response (404) from Anchore. Please verify that correct version of Anchore is deployed.")
		return ErrAnchoreEndpointDoesNotExist(path)
	}

	// Check we received a valid JSON response from Anchore, this will help catch
	// any redirect responses where it returns HTML login pages e.g. Enterprise
	// running behind cloudflare where a login page is returned with the status 200
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read %s response from Anchore: %w", operation, err)
	}
	if len(respBody) > 0 && !json.Valid(respBody) {
		log.Debug("Anchore %s response body: ", operation, string(respBody))
		return fmt.Errorf("%s response from Anchore is not valid json: %+v", operation, resp)
	}
	return nil
}
