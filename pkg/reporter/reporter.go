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
	"time"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/h2non/gock"
)

const (
	reportAPIPathV1 = "v1/enterprise/kubernetes-inventory"
	reportAPIPathV2 = "v2/kubernetes-inventory"
)

var enterpriseEndpoint = reportAPIPathV2

// This method does the actual Reporting (via HTTP) to Anchore
//
//nolint:gosec
func Post(report inventory.Report, anchoreDetails config.AnchoreInfo) error {
	defer tracker.TrackFunctionTime(time.Now(), "Reporting results to Anchore for cluster: "+report.ClusterName+"")
	log.Debug("Reporting results to Anchore using endpoint: ", enterpriseEndpoint)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	}
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
	if resp.StatusCode == 404 {
		previousVersion := enterpriseEndpoint
		// We failed to send the inventory.  We need to check the version of Enterprise.
		versionError := checkVersion(anchoreDetails)
		if versionError != nil {
			return fmt.Errorf("failed to validate Enterprise API: %w", versionError)
		}
		if previousVersion != enterpriseEndpoint {
			// We need to re-send the inventory with the new endpoint
			log.Info("Retrying inventory report with new endpoint: %s", enterpriseEndpoint)
			return Post(report, anchoreDetails)
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed to report data to Anchore: %+v", resp)
	}
	log.Debug("Successfully reported results to Anchore")
	return nil
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
//
//nolint:gosec
func checkVersion(anchoreDetails config.AnchoreInfo) error {
	log.Debug("Detecting Anchore API version")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	}
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
		fmt.Println("fff")
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
