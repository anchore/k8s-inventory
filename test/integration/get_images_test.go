package integration

import (
	"github.com/anchore/kai/cmd"
	"github.com/anchore/kai/kai"
	"strings"
	"testing"
)

const IntegrationTestNamespace = "kai-integration-test"
const IntegrationTestImageTag = "nginx:latest"

// Assumes that the hello-world helm chart in ./fixtures was installed (basic nginx container)
func TestGetImageResults(t *testing.T) {
	cmd.InitAppConfig()
	imagesResult, err := kai.GetImageResults(cmd.GetAppConfig())
	if err != nil {
		t.Fatalf("failed to get image results: %v", err)
	}

	if imagesResult.ServerVersionMetadata == nil {
		t.Errorf("Failed to include Server Version Metadata in result")
	}

	if imagesResult.Timestamp == "" {
		t.Errorf("Failed to include Timestamp in result")
	}

	foundIntegrationTestNamespace := false
	for _, namespace := range imagesResult.Results {
		if namespace.Namespace != IntegrationTestNamespace {
			continue
		} else {
			foundIntegrationTestNamespace = true
			foundIntegrationTestImage := false
			for _, image := range namespace.Images {
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
