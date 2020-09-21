package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/kai/result"
	"net/http"
)

func Report(result result.Result, anchoreDetails config.AnchoreInfo) error {
	log.Debug("Reporting results to Anchore")
	client := &http.Client{}
	// 	TODO: update path once we have an endpoint to post to
	anchoreUrl := anchoreDetails.Url + "/foo"

	reqBody, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to serialize results as JSON: %w", err)
	}

	req, err := http.NewRequest("POST", anchoreUrl, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to build request to report data to Anchore: %w", err)
	}
	req.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to report data to Anchore: %w", err)
	}
	log.Debug("Successfully reported results to Anchore")
	return nil
}
