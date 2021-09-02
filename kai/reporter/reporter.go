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

const ReportAPIPath = "/v1/images"

type ImageReport struct {
	Source Source `json:"source"`
}

type Source struct {
	Digest Digest `json:"digest"`
}

type Digest struct {
	PullString string `json:"pullstring"`
	Tag        string `json:"tag"`
	Timestamp  string `json:"creation_timestamp_override"`
}

func stripTagName(tag string) string {
	for i, v := range tag {
		if v == ':' || v == '@' {
			return tag[:i]
		}
	}
	return tag
}

func newPullString(tag, digest string) string {
	return fmt.Sprintf("docker.io/%s@%s", stripTagName(tag), digest)
}

func NewImageReport(result result.Result) (report []ImageReport) {
	for _, namespace := range result.Results {
		for _, image := range namespace.Images {
			// TODO: handle docker.io tags
			report = append(report, ImageReport{
				Source: Source{
					Digest: Digest{
						PullString: newPullString(image.Tag, image.RepoDigest),
						Tag: image.Tag,
						Timestamp: result.Timestamp,
					},
				},
			})
		}
	}
	return report
}

// This method does the actual Reporting (via HTTP) to Anchore
//nolint:gosec
func Report(result result.Result, anchoreDetails config.AnchoreInfo, appConfig *config.Application) error {
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

	for i, image := range NewImageReport(result) {
		log.Debug("Reporting image ", i)
		reqBody, err := json.Marshal(image)
		if err != nil {
			return fmt.Errorf("failed to serialize results as JSON: %w", err)
		}

		req, err := http.NewRequest("POST", anchoreURL, bytes.NewBuffer(reqBody))
		if err != nil {
			return fmt.Errorf("failed to build request to report data to Anchore: %w", err)
		}
		req.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
		req.Header.Set("Content-Type", "application/json")
		//req.Header.Set("x-anchore-account", anchoreDetails.Account)
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to report data to Anchore: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to report data to Anchore: %+v", resp)
		}
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
