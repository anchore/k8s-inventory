package reporter

import (
	"testing"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func TestBuildUrl(t *testing.T) {
	anchoreDetails := config.AnchoreInfo{
		URL:      "https://ancho.re",
		User:     "admin",
		Password: "foobar",
	}

	expectedURL := "https://ancho.re/v1/enterprise/kubernetes-inventory"
	actualURL, err := buildURL(anchoreDetails, "v1/enterprise/kubernetes-inventory")
	if err != nil || expectedURL != actualURL {
		t.Errorf("Failed to build URL:\nexpected=%s\nactual=%s", expectedURL, actualURL)
	}

	expectedURL = "https://ancho.re/v2/kubernetes-inventory"
	actualURL, err = buildURL(anchoreDetails, "v2/kubernetes-inventory")
	if err != nil || expectedURL != actualURL {
		t.Errorf("Failed to build URL:\nexpected=%s\nactual=%s", expectedURL, actualURL)
	}
}

func TestPost(t *testing.T) {
	defer gock.Off()

	type args struct {
		report         inventory.Report
		anchoreDetails config.AnchoreInfo
	}
	tests := []struct {
		name            string
		args            args
		wantErr         bool
		expectedAPIPath string
	}{
		{
			name: "default post to v2",
			args: args{
				report: inventory.Report{},
				anchoreDetails: config.AnchoreInfo{
					URL:      "https://ancho.re",
					User:     "admin",
					Password: "foobar",
					Account:  "test",
					HTTP: config.HTTPConfig{
						TimeoutSeconds: 10,
						Insecure:       true,
					},
				},
			},
			wantErr:         false,
			expectedAPIPath: reportAPIPathV2,
		},
		{
			name: "post to v1 when v2 is not found",
			args: args{
				report: inventory.Report{},
				anchoreDetails: config.AnchoreInfo{
					URL:      "https://ancho.re",
					User:     "admin",
					Password: "foobar",
					Account:  "test",
					HTTP: config.HTTPConfig{
						TimeoutSeconds: 10,
						Insecure:       true,
					},
				},
			},
			wantErr:         false,
			expectedAPIPath: reportAPIPathV1,
		},
		{
			name: "error when v1 and v2 are not found",
			args: args{
				report: inventory.Report{},
				anchoreDetails: config.AnchoreInfo{
					URL:      "https://ancho.re",
					User:     "admin",
					Password: "foobar",
					Account:  "test",
					HTTP: config.HTTPConfig{
						TimeoutSeconds: 10,
						Insecure:       true,
					},
				},
			},
			wantErr:         true,
			expectedAPIPath: reportAPIPathV1,
		},
	}
	for _, tt := range tests {
		switch tt.name {
		case "default post to v2":
			gock.New("https://ancho.re").
				Post(reportAPIPathV2).
				Reply(200)
		case "post to v1 when v2 is not found":
			gock.New("https://ancho.re").
				Post(reportAPIPathV2).
				Reply(404)
			gock.New("https://ancho.re").
				Post(reportAPIPathV1).
				Reply(200)
			gock.New("https://ancho.re").
				Get("/").
				Reply(200).
				BodyString("v1")
		case "error when v1 and v2 are not found":
			gock.New("https://ancho.re").
				Post(enterpriseEndpoint).
				Reply(404)
			gock.New("https://ancho.re").
				Get("/").
				Reply(404)
		}

		t.Run(tt.name, func(t *testing.T) {
			// Reset enterpriseEndpoint to the default each test run
			enterpriseEndpoint = reportAPIPathV2

			err := Post(tt.args.report, tt.args.anchoreDetails)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAPIPath, enterpriseEndpoint)
			}
		})
	}
}
