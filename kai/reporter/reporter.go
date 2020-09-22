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

	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/kai/result"
)

// This method does the actual Reporting (via HTTP) to Anchore
//nolint:gosec
func Report(result result.Result, anchoreDetails config.AnchoreInfo) error {
	log.Debug("Reporting results to Anchore")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}

	// 	TODO: update path once we have an endpoint to post to
	anchoreURL, err := buildURL(anchoreDetails)
	if err != nil {
		return fmt.Errorf("failed to build url: %w", err)
	}

	reqBody, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to serialize results as JSON: %w", err)
	}

	req, err := http.NewRequest("POST", anchoreURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to build request to report data to Anchore: %w", err)
	}
	req.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to report data to Anchore: %w", err)
	}
	defer resp.Body.Close()
	log.Debug("Successfully reported results to Anchore")
	return nil
}

func buildURL(anchoreDetails config.AnchoreInfo) (string, error) {
	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return "", err
	}

	anchoreURL.Path += "foo"
	return anchoreURL.String(), nil
}
