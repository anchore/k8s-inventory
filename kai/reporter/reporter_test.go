package reporter

import (
	"github.com/anchore/kai/internal/config"
	"testing"
)

func TestBuildUrl(t *testing.T) {
	anchoreDetails := config.AnchoreInfo{
		URL:      "https://ancho.re",
		User:     "admin",
		Password: "foobar",
	}

	expectedUrl := "https://ancho.re/foo"
	actualUrl, err := buildURL(anchoreDetails)
	if err != nil || expectedUrl != actualUrl {
		t.Errorf("Failed to build URL:\nexpected=%s\nactual=%s", expectedUrl, actualUrl)
	}
}
