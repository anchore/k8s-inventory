// Once In-Use Image data has been gathered, this package reports the data to Anchore
package reporter

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/h2non/gock"
)

const (
	reportAPIPathV1            = "v1/enterprise/kubernetes-inventory"
	reportAPIPathV2            = "v2/kubernetes-inventory"
	AnchoreAccountMissingError = "User account not found"
)

var (
	ErrAnchoreAccountDoesNotExist = fmt.Errorf("user account not found")
	enterpriseEndpoint            = reportAPIPathV2
)

type AnchoreResponse struct {
	Message          string      `json:"message"`
	Httpcode         int         `json:"httpcode"`
	Detail           interface{} `json:"detail"`
	AnchoreRequestID string      `json:"anchore_request_id"`
}

// This method does the actual Reporting (via HTTP) to Anchore
//
//nolint:funlen
func Post(report inventory.Report, anchoreDetails config.AnchoreInfo) error {
	defer tracker.TrackFunctionTime(time.Now(), "Reporting results to Anchore for cluster: "+report.ClusterName+"")
	log.Debug("Validating and normalizing report before sending to Anchore")
	report, modified := Normalize(report)
	if modified {
		log.Warnf("Report was modified during normalization, some data may be missing")
	}

	log.Debug("Reporting results to Anchore using endpoint: ", enterpriseEndpoint)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	} // #nosec G402
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}
	gock.InterceptClient(client) // Required to use gock for testing custom client

	anchoreURL, err := buildURL(anchoreDetails, enterpriseEndpoint)
	if err != nil {
		return fmt.Errorf("failed to build url: %w", err)
	}

	reqBody, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to serialize results as JSON: %w", err)
	}

	req, err := http.NewRequest("POST", anchoreURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to send data to Anchore: %w", err)
	}
	req.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-anchore-account", anchoreDetails.Account)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to report data to Anchore: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 403:
		log.Debug("Forbidden response (403) from Anchore")
		return ErrAnchoreAccountDoesNotExist
	case resp.StatusCode == 404:
		previousVersion := enterpriseEndpoint
		// We failed to send the inventory.  We need to check the version of Enterprise.
		versionError := checkVersion(anchoreDetails)
		if versionError != nil {
			return fmt.Errorf("failed to validate Enterprise API: %w", versionError)
		}
		if previousVersion != enterpriseEndpoint {
			// We need to re-send the inventory with the new endpoint
			log.Info("Retrying inventory report with new endpoint: ", enterpriseEndpoint)
			return Post(report, anchoreDetails)
		}

		// Check if account is correct
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response from Anchore: %w", err)
		}
		anchoreResponse := &AnchoreResponse{}
		err = json.Unmarshal(respBody, anchoreResponse)
		if err != nil {
			return fmt.Errorf("failed to parse response from Anchore: %w", err)
		}
		if strings.Contains(anchoreResponse.Message, AnchoreAccountMissingError) {
			return ErrAnchoreAccountDoesNotExist
		}
		return fmt.Errorf("failed to report data to Anchore: %s", string(respBody))
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed to report data to Anchore: %+v", resp)
	}

	// Check we received a valid JSON response from Anchore, this will help catch
	// any redirect responses where it returns HTML login pages e.g. Enterprise
	// running behind cloudflare where a login page is returned with the status 200
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response from Anchore: %w", err)
	}
	if len(respBody) > 0 && !json.Valid(respBody) {
		log.Debug("Anchore response body: ", string(respBody))
		return fmt.Errorf("failed to report data to Anchore not a valid json response: %+v", resp)
	}
	log.Debug("Successfully reported results to Anchore")
	return nil
}

// Only send a report that contains all required references in the report. E.g. if a container references a pod that is not in the report, remove the container from the report and log it.
// This is likely due to timing issues gathering the inventory but we should not send incomplete data to Anchore.
// Returns the normalized report and a boolean indicating if the report was modified.
//
//nolint:funlen
func Normalize(report inventory.Report) (inventory.Report, bool) {
	modified := false

	namespaces := make(map[string]inventory.Namespace)
	for _, ns := range report.Namespaces {
		if ns.UID == "" {
			modified = true
			log.Warnf("Namespace has no UID omitting from report: %s", ns.Name)
			continue
		}
		namespaces[ns.UID] = ns
	}

	nodes := make(map[string]inventory.Node)
	for _, node := range report.Nodes {
		if node.UID == "" {
			modified = true
			log.Warnf("Node has no UID omitting from report: %s", node.Name)
			continue
		}
		nodes[node.UID] = node
	}

	pods := make(map[string]inventory.Pod)
	for _, pod := range report.Pods {
		if pod.UID == "" {
			modified = true
			log.Warnf("Pod has no UID omitting from report: %s", pod.Name)
			continue
		}
		if _, ok := namespaces[pod.NamespaceUID]; !ok {
			modified = true
			log.Warnf(
				"Pod references a namespace that is not in the report, omitting from final report: %s, %s",
				pod.UID,
				pod.Name,
			)
			continue
		}
		if _, ok := nodes[pod.NodeUID]; !ok {
			modified = true
			log.Warnf(
				"Pod references a node that is not in the report, omitting Node field from final report: %s, %s",
				pod.NodeUID,
				pod.Name,
			)
			oldPod := pod
			pod = inventory.Pod{
				Annotations:  oldPod.Annotations,
				Labels:       oldPod.Labels,
				Name:         oldPod.Name,
				NamespaceUID: oldPod.NamespaceUID,
				UID:          oldPod.UID,
			}
		}

		pods[pod.UID] = pod
	}

	containers := make([]inventory.Container, 0)
	for _, container := range report.Containers {
		if container.ID == "" {
			modified = true
			log.Warnf("Container has no ID omitting from report: %s", container.Name)
			continue
		}
		if _, ok := pods[container.PodUID]; !ok {
			modified = true
			log.Warnf("Container references a pod that is not in the report: %s, %s", container.ID, container.Name)
			continue
		}

		containers = append(containers, container)
	}

	// Create a new report with only the objects that have all references in the report
	newReport := inventory.Report{
		ClusterName:           report.ClusterName,
		Containers:            containers,
		Namespaces:            make([]inventory.Namespace, 0),
		Nodes:                 make([]inventory.Node, 0),
		Pods:                  make([]inventory.Pod, 0),
		ServerVersionMetadata: report.ServerVersionMetadata,
		Timestamp:             report.Timestamp,
	}

	for _, ns := range namespaces {
		newReport.Namespaces = append(newReport.Namespaces, ns)
	}
	for _, node := range nodes {
		newReport.Nodes = append(newReport.Nodes, node)
	}
	for _, pod := range pods {
		newReport.Pods = append(newReport.Pods, pod)
	}
	return newReport, modified
}

type AnchoreVersion struct {
	API struct {
		Version string `json:"version"`
	} `json:"api"`
	DB struct {
		SchemaVersion string `json:"schema_version"`
	} `json:"db"`
	Service struct {
		Version string `json:"version"`
	} `json:"service"`
}

// This method retrieves the API version from Anchore
// and caches the response if parsed successfully
func checkVersion(anchoreDetails config.AnchoreInfo) error {
	log.Debug("Detecting Anchore API version")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	} // #nosec G402
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}
	gock.InterceptClient(client) // Required to use gock for testing custom client

	resp, err := client.Get(anchoreDetails.URL + "/version")
	if err != nil {
		return fmt.Errorf("failed to contact Anchore API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to retrieve Anchore API version: %+v", resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Anchore API version: %w", err)
	}

	ver := AnchoreVersion{}
	err = json.Unmarshal(body, &ver)
	if err != nil {
		return fmt.Errorf("failed to parse API version: %w", err)
	}

	log.Debug("Anchore API version: ", ver)
	if ver.API.Version == "2" {
		enterpriseEndpoint = reportAPIPathV2
	} else {
		// If we can't parse the version, we'll assume it's v1 as 4.X does not include the version in the API version response
		enterpriseEndpoint = reportAPIPathV1
	}

	log.Info("Using enterprise endpoint ", enterpriseEndpoint)
	return nil
}

func buildURL(anchoreDetails config.AnchoreInfo, path string) (string, error) {
	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return "", err
	}

	anchoreURL.Path += path

	return anchoreURL.String(), nil
}
