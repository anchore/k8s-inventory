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

const ReportAPIPath = "v1/enterprise/kubernetes-inventory"

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

func buildURL(anchoreDetails config.AnchoreInfo) (string, error) {
	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return "", err
	}

	anchoreURL.Path += ReportAPIPath

	return anchoreURL.String(), nil
}
