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
	"strconv"
	"time"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/inventory"
)

const ReportAPIPathV1 = "v1/enterprise/kubernetes-inventory"
const ReportAPIPathV2 = "v2/kubernetes-inventory"

var Version = 0

// This method does the actual Reporting (via HTTP) to Anchore
//
//nolint:gosec
func Post(report inventory.Report, anchoreDetails config.AnchoreInfo) error {
	defer tracker.TrackFunctionTime(time.Now(), "Reporting results to Anchore for cluster: "+report.ClusterName+"")
	log.Debug("Reporting results to Anchore")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}

	anchoreURL, err := buildURL(anchoreDetails)
	if err != nil {
		return fmt.Errorf("failed to build url: %w", err)
	}

	reqBody, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to serialize results as JSON: %w", err)
	}

	req, err := http.NewRequest("POST", anchoreURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to build request to report data to Anchore: %w", err)
	}
	req.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-anchore-account", anchoreDetails.Account)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to report data to Anchore: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed to report data to Anchore: %+v", resp)
	}
	log.Debug("Successfully reported results to Anchore")
	return nil
}

type anchoreVersion struct {
	Api struct {
		Version string `json:"version"`
	} `json:"api"`
}

// This method retrieves the API version from Anchore
//
//nolint:gosec
func getVersion(anchoreDetails config.AnchoreInfo) (int, error) {
	log.Debug("Detecting Anchore API version")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}
	resp, err := client.Get(anchoreDetails.URL + "/version")
	if err != nil {
		return 0, fmt.Errorf("failed to request API version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, fmt.Errorf("failed to retrieve API version: %+v", resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read API version: %w", err)
	}

	ver := anchoreVersion{}
	err = json.Unmarshal(body, &ver)
	if err != nil {
		return 0, fmt.Errorf("failed to parse API version: %w", err)
	}

	return strconv.Atoi(ver.Api.Version)
}

func buildURL(anchoreDetails config.AnchoreInfo) (string, error) {
	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return "", err
	}

	if Version == 0 {
		Version, err = getVersion(anchoreDetails)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve API version: %w", err)
		}
	}

	if Version == 1 {
		anchoreURL.Path += ReportAPIPathV1
	} else {
		anchoreURL.Path += ReportAPIPathV2
	}

	return anchoreURL.String(), nil
}
