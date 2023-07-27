package reporter

import (
	"testing"

	"github.com/anchore/k8s-inventory/internal/config"
)

func TestBuildUrl(t *testing.T) {
	anchoreDetails := config.AnchoreInfo{
		URL:      "https://ancho.re",
		User:     "admin",
		Password: "foobar",
	}

	Version = 1
	expectedURL := "https://ancho.re/v1/enterprise/kubernetes-inventory"
	actualURL, err := buildURL(anchoreDetails)
	if err != nil || expectedURL != actualURL {
		t.Errorf("Failed to build URL:\nexpected=%s\nactual=%s", expectedURL, actualURL)
	}

	Version = 2
	expectedURL = "https://ancho.re/v2/kubernetes-inventory"
	actualURL, err = buildURL(anchoreDetails)
	if err != nil || expectedURL != actualURL {
		t.Errorf("Failed to build URL:\nexpected=%s\nactual=%s", expectedURL, actualURL)
	}
}
