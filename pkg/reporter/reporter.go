// Once In-Use Image data has been gathered, this package reports the data to Anchore
package reporter

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/inventory"
)

const reportAPIPathV1 = "v1/enterprise/kubernetes-inventory"
const reportAPIPathV2 = "v2/kubernetes-inventory"

var cachedVersion = 0

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

	version, err := getVersion(anchoreDetails)
	if err != nil {
		return err
	}

	anchoreURL, err := buildURL(anchoreDetails, version)
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

// This method retrieves the API version from Anchore
// and caches the response if parsed successfully
//
//nolint:gosec
func getVersion(anchoreDetails config.AnchoreInfo) (int, error) {
	if cachedVersion > 0 {
		return cachedVersion, nil
	}

	log.Debug("Detecting Anchore API version")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}

	// attempt to retrieve v2/ endpoint, if successful we are V2
	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return 0, err
	}
	anchoreURL.Path += "v2/"

	resp, err := client.Get(anchoreURL.String())
	if err != nil {
		return 0, fmt.Errorf("failed to contact Anchore API: %w", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		log.Debug("Detected Anchore APIv2")
		cachedVersion = 2
	} else {
		log.Debug("Detected Anchore APIv1")
		cachedVersion = 1
	}

	return cachedVersion, nil
}

func buildURL(anchoreDetails config.AnchoreInfo, version int) (string, error) {
	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return "", err
	}

	if version == 1 {
		anchoreURL.Path += reportAPIPathV1
	} else {
		anchoreURL.Path += reportAPIPathV2
	}

	return anchoreURL.String(), nil
}
