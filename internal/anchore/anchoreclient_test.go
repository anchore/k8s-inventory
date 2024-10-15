package anchore

import (
	"fmt"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"testing"
)

type httpError struct {
	err     string
	timeout bool
}

func (e *httpError) Error() string { return e.err }
func (e *httpError) Timeout() bool { return e.timeout }

var (
	version = map[string]interface{}{
		"service": map[string]interface{}{
			"version": "5.11.0",
		},
		"api": map[string]interface{}{
			"version": "2",
		},
		"db": map[string]interface{}{
			"schema_version": "5110",
		},
	}

	versionObj = Version{
		API: struct {
			Version string `json:"version"`
		}(struct {
			Version string
		}{"2"}),
		DB: struct {
			SchemaVersion string `json:"schema_version"`
		}(struct {
			SchemaVersion string
		}{"5110"}),
		Service: struct {
			Version string `json:"version"`
		}(struct {
			Version string
		}{"5.11.0"}),
	}

	integration = map[string]interface{}{
		"uuid":               "000d1e60-cb05-4cce-8d1e-60cb052cce1f",
		"type":               "k8s_inventory_agent",
		"name":               "k8s-inv-agent",
		"description":        "k8s-agent with health reporting",
		"version":            "2.0",
		"reported_status":    map[string]interface{}{"state": "HEALTHY"},
		"integration_status": map[string]interface{}{"state": "REGISTERED"},
		"started_at":         "2024-04-10T12:14:16Z",
		// "last_seen": nil
		"uptime":                   "2.04",
		"username":                 "admin",
		"account_name":             "admin",
		"explicitly_account_bound": []interface{}{},
		"accounts":                 []interface{}{},
		"namespaces":               []interface{}{},
		"cluster_name":             "Docker-Desktop",
		"namespace":                "default",
		"health_report_interval":   60,
		"registration_id":          "de2c3c58-4c20-4d87-ac3c-584c201d875a",
		"registration_instance_id": "45743315",
	}

	errOther = fmt.Errorf("other errorMsg")

	connectionTimeoutError = url.Error{
		Op:  "Post",
		URL: "http://127.0.0.1:8228/v2/system/integrations/registration",
		Err: &httpError{err: "net/http: timeout awaiting response headers", timeout: true},
	}

	connectionRefusedError = url.Error{
		Op:  "Post",
		URL: "http://127.0.0.1:8228/v2/system/integrations/registration",
		Err: &net.OpError{
			Op:     "dial",
			Net:    "tcp",
			Source: nil,
			Addr: &net.TCPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 8228,
				Zone: "",
			},
			Err: &os.SyscallError{
				Syscall: "connect",
				Err:     syscall.ECONNREFUSED,
			},
		},
	}

	connectionResetError = url.Error{
		Op:  "Post",
		URL: "http://127.0.0.1:8228/v2/system/integrations/registration",
		Err: &net.OpError{
			Op:  "read",
			Net: "tcp",
			Source: &net.TCPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 62122,
				Zone: "",
			},
			Addr: &net.TCPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 8228,
				Zone: "",
			},
			Err: &os.SyscallError{
				Syscall: "read",
				Err:     syscall.ECONNRESET,
			},
		},
	}

	badGatewayError = APIClientError{
		HTTPStatusCode: http.StatusBadGateway,
		Message:        "Bad Gateway",
		Path:           "/v2/system/integrations/registration",
		Method:         "POST",
	}

	serviceUnavailableError = APIClientError{
		HTTPStatusCode: http.StatusServiceUnavailable,
		Message:        "Service Unavailable",
		Path:           "/v2/system/integrations/registration",
		Method:         "POST",
	}

	gatewayTimeoutError = APIClientError{
		HTTPStatusCode: http.StatusGatewayTimeout,
		Message:        "Gateway Timeout",
		Path:           "/v2/system/integrations/registration",
		Method:         "POST",
	}

	urlNotFoundError = APIClientError{
		HTTPStatusCode: http.StatusNotFound,
		Message:        "404 Not Found response from Anchore (during integration registration)",
		Path:           "/v2/system/integrations/registration",
		Method:         "POST",
		ControllerErrorDetails: &ControllerErrorDetails{
			Type:   "about:blank",
			Title:  "Not Found",
			Detail: "The requested URL was not found on the server. If you entered the URL manually please check your spelling and try again.",
			Status: http.StatusNotFound,
		},
	}

	methodNotAllowedError = APIClientError{
		HTTPStatusCode: http.StatusMethodNotAllowed,
		Message:        "405 Method Not Allowed response from Anchore (during integration registration)",
		Path:           "/v2/system/integrations/registration",
		Method:         "POST",
		ControllerErrorDetails: &ControllerErrorDetails{
			Type:   "about:blank",
			Title:  "Method Not Allowed",
			Detail: "Method Not Allowed",
			Status: http.StatusMethodNotAllowed,
		},
	}

	unAuthorizedError = APIClientError{
		HTTPStatusCode: http.StatusUnauthorized,
		Message:        "401 Unauthorized response from Anchore (during integration registration)",
		Path:           "/v2/system/integrations/registration",
		Method:         "POST",
	}

	insufficientPrivilegeError = APIClientError{
		HTTPStatusCode: http.StatusForbidden,
		Message:        "403 Forbidden response from Anchore (during integration registration)",
		Path:           "/v2/system/integrations/registration",
		Method:         "POST",
		APIErrorDetails: &APIErrorDetails{
			Message:  "Not authorized. Requires permissions: domain=account0 action=registerIntegration target=",
			Detail:   map[string]interface{}{},
			HTTPCode: http.StatusForbidden,
		},
	}

	anchoreDetails = config.AnchoreInfo{
		URL:      "https://ancho.re",
		User:     "admin",
		Password: "foobar",
		Account:  "account0",
	}
)

func TestGetVersion(t *testing.T) {
	defer gock.Off()

	type args struct {
		anchoreDetails config.AnchoreInfo
	}
	tests := []struct {
		name    string
		args    args
		want    *Version
		wantErr bool
	}{
		{
			name: "successful get version",
			args: args{
				anchoreDetails: anchoreDetails,
			},
			want:    &versionObj,
			wantErr: false,
		},
		{
			name: "bad json response",
			args: args{
				anchoreDetails: anchoreDetails,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "other error",
			args: args{
				anchoreDetails: anchoreDetails,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing",
			args: args{
				anchoreDetails: anchoreDetails,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		gock.Flush()

		switch tt.name {
		case "successful get version":
			gock.New("https://ancho.re").
				Get("/version").
				Reply(200).
				JSON(version)
		case "bad json response":
			gock.New("https://ancho.re").
				Get("/version").
				Reply(200).
				BodyString("bad json")
		case "other error":
			gock.New("https://ancho.re").
				Get("/version").
				Reply(http.StatusBadRequest)
		case "missing":
			gock.New("https://ancho.re").
				Get("/vversion").
				Reply(200)
		}
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetVersion(tt.args.anchoreDetails)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestPost(t *testing.T) {
	defer gock.Off()

	type args struct {
		requestBody    []byte
		id             string
		path           string
		anchoreDetails config.AnchoreInfo
		operation      string
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "successful registration",
			args: args{
				requestBody:    []byte(`{"id":"1"}`),
				id:             "",
				path:           "v2/system/integrations/registration",
				anchoreDetails: anchoreDetails,
				operation:      "integration registration",
			},
		},
		{
			name: "401 error",
			args: args{
				requestBody:    []byte(`{"id":"1"}`),
				id:             "",
				path:           "v2/system/integrations/registration",
				anchoreDetails: anchoreDetails,
				operation:      "integration registration",
			},
			wantErr: &unAuthorizedError,
		},
		{
			name: "403 error",
			args: args{
				requestBody:    []byte(`{"id":"1"}`),
				id:             "",
				path:           "v2/system/integrations/registration",
				anchoreDetails: anchoreDetails,
				operation:      "integration registration",
			},
			wantErr: &insufficientPrivilegeError,
		},
		{
			name: "404 error",
			args: args{
				requestBody:    []byte(`{"id":"1"}`),
				id:             "",
				path:           "v2/system/integrations/registration",
				anchoreDetails: anchoreDetails,
				operation:      "integration registration",
			},
			wantErr: &urlNotFoundError,
		},
	}
	for _, tt := range tests {
		switch tt.name {
		case "successful registration":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(200).
				JSON(integration)
		case "401 error":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(401)
		case "403 error":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(403).
				JSON(map[string]interface{}{
					"message":  "Not authorized. Requires permissions: domain=account0 action=registerIntegration target=",
					"detail":   map[string]interface{}{},
					"httpcode": http.StatusForbidden,
				})
		case "404 error":
			gock.New("https://ancho.re").
				Post("v2/system/integrations/registration").
				Reply(404).
				JSON(map[string]interface{}{
					"type":   "about:blank",
					"title":  "Not Found",
					"detail": "The requested URL was not found on the server. If you entered the URL manually please check your spelling and try again.",
					"status": http.StatusNotFound,
				})
		}
		t.Run(tt.name, func(t *testing.T) {
			result, err := Post(tt.args.requestBody, tt.args.id, tt.args.path, tt.args.anchoreDetails, tt.args.operation)
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestGetUrl(t *testing.T) {
	type args struct {
		anchoreDetails config.AnchoreInfo
		url            string
		uuid           string
	}
	type want struct {
		expectedURL string
		errorMsg    string
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Registration url",
			args: args{
				anchoreDetails: anchoreDetails,
				url:            "v2/system/integrations/registration",
				uuid:           "",
			},
			want: want{
				expectedURL: "https://ancho.re/v2/system/integrations/registration",
				errorMsg:    "",
			},
		},
		{
			name: "Health report url",
			args: args{
				anchoreDetails: anchoreDetails,
				url:            "v2/system/integrations/{{id}}/health-report",
				uuid:           "0ec44439-d091-4bf2-8444-39d0916bf220",
			},
			want: want{
				expectedURL: "https://ancho.re/v2/system/integrations/0ec44439-d091-4bf2-8444-39d0916bf220/health-report",
				errorMsg:    "",
			},
		},
		{
			name: "faulty url",
			args: args{
				anchoreDetails: config.AnchoreInfo{
					URL:      "htt$ps://ancho.re",
					User:     "admin",
					Password: "foobar",
				},
				url:  "v2/system/integrations/{{id}}/health-report",
				uuid: "0ec44439-d091-4bf2-8444-39d0916bf220",
			},
			want: want{
				expectedURL: "",
				errorMsg:    "failed to build path (v2/system/integrations/{{id}}/health-report)",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getURL(tt.args.anchoreDetails, tt.args.url, tt.args.uuid)
			assert.Equal(t, tt.want.expectedURL, result)
			if tt.want.errorMsg != "" {
				assert.ErrorContains(t, err, tt.want.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetPostRequest(t *testing.T) {
	type args struct {
		url     string
		reqBody []byte
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Good path",
			args: args{
				url:     "http://localhost:8228/v2/system/integrations/registration",
				reqBody: make([]byte, 0),
			},
			want: nil,
		},
		{
			name: "Bad path",
			args: args{
				url:     "_http://localhost:8228/v2/system/integrations/registration",
				reqBody: nil,
			},
			want: &url.Error{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getPostRequest(anchoreDetails, tt.args.url, tt.args.reqBody, "register integration")
			if tt.want != nil {
				assert.Nil(t, result)
				assert.Error(t, err, tt.want)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, result.Header.Get("content-type"), "application/json")
				assert.Contains(t, result.Header.Get("Authorization"), "Basic")
				assert.Contains(t, result.Header.Get("X-Anchore-Account"), "account0")
			}
		})
	}
}

func TestAnchoreIsOffline(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Connection timeout returns true",
			err:  &connectionTimeoutError,
			want: true,
		},
		{
			name: "Connection refused errorMsg returns true",
			err:  &connectionRefusedError,
			want: true,
		},
		{
			name: "Connection reset errorMsg returns true",
			err:  &connectionResetError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with 502 http_status returns true",
			err:  &badGatewayError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with 503 http_status returns true",
			err:  &serviceUnavailableError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with 504 http_status returns true",
			err:  &gatewayTimeoutError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with 401 http_status returns false",
			err:  &unAuthorizedError,
			want: false,
		},
		{
			name: "Other errorMsg returns false",
			err:  errOther,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ServerIsOffline(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestAnchoreLacksAgentHealthAPISupport(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "AnchoreAPIClientError with 404 http_status returns true",
			err:  &urlNotFoundError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with 405 http_status returns true",
			err:  &methodNotAllowedError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with 401 http_status returns false",
			err:  &unAuthorizedError,
			want: false,
		},
		{
			name: "Other errorMsg returns false",
			err:  errOther,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ServerLacksAgentHealthAPISupport(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestUserLacksAPIPrivileges(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "AnchoreAPIClientError with 403 http_status returns true",
			err:  &insufficientPrivilegeError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with 401 http_status returns false",
			err:  &unAuthorizedError,
			want: false,
		},
		{
			name: "Other errorMsg returns false",
			err:  errOther,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UserLacksAPIPrivileges(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestIncorrectCredentials(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "AnchoreAPIClientError with 401 http_status returns true",
			err:  &unAuthorizedError,
			want: true,
		},
		{
			name: "AnchoreAPIClientError with non 403 http_status returns false",
			err:  &insufficientPrivilegeError,
			want: false,
		},
		{
			name: "Other errorMsg returns false",
			err:  errOther,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IncorrectCredentials(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}
