package anchore

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/h2non/gock"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

type Version struct {
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

type ControllerErrorDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

type APIErrorDetails struct {
	Message  string                 `json:"message"`
	Detail   map[string]interface{} `json:"detail"`
	HTTPCode int                    `json:"httpcode"`
}

type APIClientError struct {
	HTTPStatusCode         int
	Message                string
	Path                   string
	Method                 string
	Body                   *[]byte
	APIErrorDetails        *APIErrorDetails
	ControllerErrorDetails *ControllerErrorDetails
}

func (e *APIClientError) Error() string {
	return fmt.Sprintf("API errorMsg(%d): %s Path: %q %v %v", e.HTTPStatusCode, e.Message, e.Path,
		e.APIErrorDetails, e.ControllerErrorDetails)
}

func GetVersion(anchoreDetails config.AnchoreInfo) (*Version, error) {
	operation := "version get"
	defer tracker.TrackFunctionTime(time.Now(), fmt.Sprintf("Sent %s request to Anchore", operation))

	log.Debug("Determining Anchore service version")

	client := getClient(anchoreDetails)

	response, err := client.Get(anchoreDetails.URL + "/version")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = checkHTTPErrors(response, operation)
	if err != nil {
		return nil, err
	}

	responseBody, err := getBody(response, operation)
	if err != nil {
		return nil, err
	}

	ver := Version{}
	err = json.Unmarshal(*responseBody, &ver)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API version: %w", err)
	}
	return &ver, nil
}

func Post(requestBody []byte, id string, path string, anchoreDetails config.AnchoreInfo, operation string) (*[]byte, error) {
	defer tracker.TrackFunctionTime(time.Now(), fmt.Sprintf("Sent %s request to Anchore", operation))

	log.Debugf("Performing %s to Anchore using endpoint: %s", operation, strings.Replace(path, "{{id}}", id, 1))

	client := getClient(anchoreDetails)

	anchoreURL, err := getURL(anchoreDetails, path, id)
	if err != nil {
		return nil, err
	}

	request, err := getPostRequest(anchoreDetails, anchoreURL, requestBody, operation)
	if err != nil {
		return nil, err
	}

	return doPost(client, request, operation)
}

func getClient(anchoreDetails config.AnchoreInfo) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	} // #nosec G402

	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}
	gock.InterceptClient(client) // Required to use gock for testing custom client

	return client
}

func getURL(anchoreDetails config.AnchoreInfo, path string, id string) (string, error) {
	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return "", fmt.Errorf("failed to build path (%s) url: %w", path, err)
	}

	anchoreURL.Path += strings.Replace(path, "{{id}}", id, 1)
	return anchoreURL.String(), nil
}

func getPostRequest(anchoreDetails config.AnchoreInfo, endpointURL string, reqBody []byte, operation string) (*http.Request, error) {
	request, err := http.NewRequest("POST", endpointURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare %s request to Anchore: %w", operation, err)
	}

	request.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-anchore-account", anchoreDetails.Account)
	return request, nil
}

func doPost(client *http.Client, request *http.Request, operation string) (*[]byte, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = checkHTTPErrors(response, operation)
	if err != nil {
		return nil, err
	}

	responseBody, err := getBody(response, operation)
	return responseBody, err
}

func checkHTTPErrors(response *http.Response, operation string) error {
	switch {
	case response.StatusCode >= 400 && response.StatusCode <= 599:
		msg := fmt.Sprintf("%s response from Anchore (during %s)", response.Status, operation)
		log.Errorf(msg)

		respBody, _ := getBody(response, operation)
		if respBody == nil {
			return &APIClientError{Message: msg, Path: response.Request.URL.Path, Method: response.Request.Method,
				Body: nil, HTTPStatusCode: response.StatusCode}
		}

		// Depending on where an errorMsg is discovered during request processing on the server, the
		// errorMsg information in the response will be either an APIErrorDetails or a ControllerErrorDetails
		apiError := APIErrorDetails{}
		err := json.Unmarshal(*respBody, &apiError)
		if err == nil {
			return &APIClientError{Message: msg, Path: response.Request.URL.Path, Method: response.Request.Method,
				Body: nil, HTTPStatusCode: response.StatusCode, APIErrorDetails: &apiError}
		}

		controllerError := ControllerErrorDetails{}
		err = json.Unmarshal(*respBody, &controllerError)
		if err == nil {
			return &APIClientError{Message: msg, Path: response.Request.URL.Path, Method: response.Request.Method,
				Body: nil, HTTPStatusCode: response.StatusCode, ControllerErrorDetails: &controllerError}
		}

		return &APIClientError{Message: msg, Path: response.Request.URL.Path, Method: response.Request.Method,
			Body: nil, HTTPStatusCode: response.StatusCode}
	case response.StatusCode < 200 || response.StatusCode > 299:
		msg := fmt.Sprintf("failed to perform %s to Anchore: %+v", operation, response)
		log.Debugf(msg)
		return &APIClientError{Message: msg, Path: response.Request.URL.Path, Method: response.Request.Method,
			Body: nil, HTTPStatusCode: response.StatusCode}
	}
	return nil
}

func getBody(response *http.Response, operation string) (*[]byte, error) {
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read %s response body from Anchore:", operation)
		log.Debugf("%s %v", operation, errMsg)
		return nil, fmt.Errorf("%s %w", errMsg, err)
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

func ServerIsOffline(err error) bool {
	if os.IsTimeout(err) {
		return true
	}

	offlineErrors := []error{
		syscall.ENETDOWN,
		syscall.ENETUNREACH,
		syscall.ENETRESET,
		syscall.ECONNABORTED,
		syscall.ECONNRESET,
		syscall.ETIMEDOUT,
		syscall.ECONNREFUSED,
		syscall.EHOSTDOWN,
		syscall.EHOSTUNREACH,
	}

	for _, e := range offlineErrors {
		if errors.Is(err, e) {
			return true
		}
	}

	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return true
	}

	var apiClientError *APIClientError
	if errors.As(err, &apiClientError) {
		if apiClientError.HTTPStatusCode == http.StatusBadGateway ||
			apiClientError.HTTPStatusCode == http.StatusServiceUnavailable ||
			apiClientError.HTTPStatusCode == http.StatusGatewayTimeout {
			return true
		}
	}

	return false
}

func ServerLacksAgentHealthAPISupport(err error) bool {
	var apiClientError *APIClientError
	if errors.As(err, &apiClientError) {
		if apiClientError.ControllerErrorDetails == nil {
			return false
		}

		if apiClientError.HTTPStatusCode == http.StatusNotFound &&
			strings.Contains(apiClientError.ControllerErrorDetails.Detail, "The requested URL was not found") {
			return true
		}

		if apiClientError.HTTPStatusCode == http.StatusMethodNotAllowed &&
			apiClientError.ControllerErrorDetails.Detail == "Method Not Allowed" {
			return true
		}
	}

	return false
}

func UserLacksAPIPrivileges(err error) bool {
	var apiClientError *APIClientError

	if errors.As(err, &apiClientError) {
		if apiClientError.APIErrorDetails == nil {
			return false
		}

		if apiClientError.HTTPStatusCode == http.StatusForbidden &&
			strings.Contains(apiClientError.APIErrorDetails.Message, "Not authorized. Requires permissions") {
			return true
		}
	}
	return false
}

func IncorrectCredentials(err error) bool {
	// This covers user that does not exist or incorrect password for user
	var apiClientError *APIClientError

	if errors.As(err, &apiClientError) && apiClientError.HTTPStatusCode == http.StatusUnauthorized {
		return true
	}

	return false
}
