package anchore

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/h2non/gock"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AnchoreAPIError struct {
	Message  string                 `json:"message"`
	Detail   map[string]interface{} `json:"detail"`
	HttpCode int                    `json:"httpcode"`
}

func ErrAnchoreEndpointDoesNotExist(path string) error {
	return fmt.Errorf("api endpoint does not exist: %s", path)
}

type ErrAchoreAPIClient struct {
	HttpStatusCode int
	Message        string
	Path           string
	Body           *[]byte
}

func (e *ErrAchoreAPIClient) Error() string {
	return fmt.Sprintf("API error(%d): %s Path: %q", e.HttpStatusCode, e.Message, e.Path)
}

func Put(requestBody []byte, id string, path string, anchoreDetails config.AnchoreInfo, operation string) (*[]byte, error) {
	defer tracker.TrackFunctionTime(time.Now(), fmt.Sprintf("Sent %s request to Anchore", operation))

	log.Debugf("Performing %s to Anchore using endpoint: %s", operation, strings.Replace(path, "{{id}}", id, 1))

	client, err := getClient(anchoreDetails)
	if err != nil {
		return nil, err
	}

	anchoreURL, err := getURL(anchoreDetails, path, id)
	if err != nil {
		return nil, err
	}

	request, err := getPutRequest(anchoreDetails, anchoreURL, requestBody, operation)
	if err != nil {
		return nil, err
	}

	return doPut(client, request, strings.Replace(path, "{{id}}", id, 1), operation)
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

func doPut(client *http.Client, request *http.Request, path string, operation string) (*[]byte, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send %s to Anchore: %w", operation, err)
	}
	defer response.Body.Close()

	err = checkHttpErrors(response, request, path, operation)
	if err != nil {
		return nil, err
	}

	responseBody, _ := getBody(response, operation)
	return responseBody, nil
}

func checkHttpErrors(response *http.Response, request *http.Request, path string, operation string) error {
	switch {
	case response.StatusCode == 403:
		msg := fmt.Sprintf("forbidden response (403) from Anchore (during %s)", operation)
		log.Debug(msg)
		return &ErrAchoreAPIClient{Message: msg, Path: path, Body: nil, HttpStatusCode: response.StatusCode}
		//return fmt.Errorf("user account not found")
	case response.StatusCode == 404:
		msg := fmt.Sprintf("forbidden response (404) from Anchore (during %s)", operation)
		log.Debugf("%s: path: %s. Please verify that correct version of Anchore is deployed.", msg, path)
		return &ErrAchoreAPIClient{Message: msg, Path: path, Body: nil, HttpStatusCode: response.StatusCode}
		//return ErrAnchoreEndpointDoesNotExist(path)
	case response.StatusCode == 409:
		msg := fmt.Sprintf("conflict response (409) from Anchore (during %s)", operation)
		log.Debug(msg)
		respBody, _ := getBody(response, operation)
		return &ErrAchoreAPIClient{Message: msg, Path: path, Body: respBody, HttpStatusCode: response.StatusCode}
	case response.StatusCode < 200 || response.StatusCode > 299:
		msg := fmt.Sprintf("failed to perform %s to Anchore: %+v", operation, response)
		log.Debugf(msg)
		return &ErrAchoreAPIClient{Message: msg, Path: path, Body: nil, HttpStatusCode: response.StatusCode}
		//return fmt.Errorf("failed to perform %s to Anchore: %+v", operation, response)

	}
	return nil
}

func getBody(response *http.Response, operation string) (*[]byte, error) {
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		err_msg := fmt.Sprintf("failed to read %s response body from Anchore:", operation)
		log.Debugf("%s %v", operation, err_msg)
		return nil, fmt.Errorf("%s %w", err_msg, err)
	}

	// Check we received a valid JSON response from Anchore, this will help catch
	// any redirect responses where it returns HTML login pages e.g. Enterprise
	// running behind cloudflare where a login page is returned with the status 200
	if len(responseBody) > 0 && !json.Valid(responseBody) {
		log.Debugf("Anchore %s response body: %s", operation, string(responseBody))
		return nil, fmt.Errorf("%s response from Anchore is not valid json: %+v", operation, response)
	}
	return &responseBody, nil
}
