package integration

import (
	"strings"
	"testing"

	"github.com/anchore/k8s-inventory/cmd"
	"github.com/anchore/k8s-inventory/pkg"
)

const (
	IntegrationTestNamespace = "k8s-inventory-integration-test"
	IntegrationTestImageTag  = "nginx:latest"
)

// Assumes that the hello-world helm chart in ./fixtures was installed (basic nginx container)
func TestGetImageResults(t *testing.T) {
	cmd.InitAppConfig()
	report, err := pkg.GetInventoryReport(cmd.GetAppConfig())
	if err != nil {
		t.Fatalf("failed to get image results: %v", err)
	}

	if report.ServerVersionMetadata == nil {
		t.Errorf("Failed to include Server Version Metadata in report")
	}

	if report.Timestamp == "" {
		t.Errorf("Failed to include Timestamp in report")
	}

	foundIntegrationTestNamespace := false
	for _, item := range report.Results {
		if item.Namespace != IntegrationTestNamespace {
			continue
		} else {
			foundIntegrationTestNamespace = true
			foundIntegrationTestImage := false
			for _, image := range item.Images {
				if !strings.Contains(image.Tag, IntegrationTestImageTag) {
					continue
				} else {
					foundIntegrationTestImage = true
					if image.RepoDigest == "" {
						t.Logf("Image Found, but no digest located: %v", image)
					}
				}
			}
			if !foundIntegrationTestImage {
				t.Errorf("failed to locate integration test image")
			}
		}
	}
	if !foundIntegrationTestNamespace {
		t.Errorf("failed to locate integration test namespace")
	}
}
