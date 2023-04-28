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
	for _, item := range report.Namespaces {
		if item.Name != IntegrationTestNamespace {
			continue
		}
		foundIntegrationTestNamespace = true
		foundIntegrationTestImage := false
		for _, image := range report.Containers {
			if !strings.Contains(image.ImageTag, IntegrationTestImageTag) {
				continue
			}
			foundIntegrationTestImage = true
			if image.ImageDigest == "" {
				t.Logf("Image Found, but no digest located: %v", image)
			}
		}
		if !foundIntegrationTestImage {
			t.Errorf("failed to locate integration test image")
		}
	}
	if !foundIntegrationTestNamespace {
		t.Errorf("failed to locate integration test namespace")
	}
}
