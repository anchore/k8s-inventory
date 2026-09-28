package applications

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"
)

const (
	testURL       = "https://ancho.re"
	testAppID     = "11111111-1111-1111-1111-111111111111"
	testVersionID = "22222222-2222-2222-2222-222222222222"
	testOldVerID  = "33333333-3333-3333-3333-333333333333"
	testOlderID   = "44444444-4444-4444-4444-444444444444"
	appName       = "cluster1/default/web"
	versionName   = "revision-2-abc123"
	assetJobPath  = "/v2/apps/" + testAppID + "/jobs/add-container-image-asset"
	versionsPath  = "/v2/apps/" + testAppID + "/versions"
	assetsPath    = "/v2/apps/" + testAppID + "/versions/" + testVersionID + "/assets"
)

var released = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func testConfig() *config.Application {
	return &config.Application{
		CreateApplicationsFromDeployments: true,
		KubeConfig:                        config.KubeConf{Cluster: "cluster1"},
		AnchoreDetails: config.AnchoreInfo{
			URL:      testURL,
			User:     "admin",
			Password: "foobar",
			Account:  "admin",
			HTTP:     config.HTTPConfig{TimeoutSeconds: 10},
		},
	}
}

func testDetails() config.AnchoreInfo {
	return testConfig().AnchoreDetails
}

func testDeployment() inventory.Deployment {
	return inventory.Deployment{
		Name:              "web",
		Namespace:         "default",
		Revision:          "2",
		ReplicaSetName:    "web-abc123",
		PodTemplateHash:   "abc123",
		ReplicaSetCreated: released,
		Containers: []inventory.Container{
			{Name: "nginx", ImageTag: "docker.io/nginx:1.27", ImageDigest: "sha256:aaaa"},
			{Name: "sidecar", ImageTag: "docker.io/busybox:1.36"},
		},
	}
}

func appJSON(id, name string) map[string]interface{} {
	return map[string]interface{}{
		"name":            name,
		"system_metadata": map[string]interface{}{"id": id, "created_at": "2026-01-01T00:00:00Z"},
	}
}

func list(items ...interface{}) map[string]interface{} {
	return map[string]interface{}{
		"pagination": map[string]interface{}{"item_count": len(items)},
		"items":      items,
	}
}

func assetJob(id string) map[string]interface{} {
	return map[string]interface{}{"status": "pending", "system_metadata": map[string]interface{}{"id": id}}
}

func mockFindApp(items ...interface{}) {
	gock.New(testURL).Get("/v2/apps").MatchParam("name", "^"+appName+"$").
		MatchHeader("x-anchore-account", "admin").Reply(200).JSON(list(items...))
}

func mockFindVersion(items ...interface{}) {
	gock.New(testURL).Get(versionsPath).MatchParam("name", "^"+versionName+"$").Reply(200).JSON(list(items...))
}

func mockListAssets(names ...string) {
	items := make([]interface{}, 0, len(names))
	for _, n := range names {
		items = append(items, map[string]interface{}{"name": n, "type": "container"})
	}
	gock.New(testURL).Get(assetsPath).Reply(200).JSON(list(items...))
}

type capturedAsset map[string]string

// mockAddAsset registers an add-container-image-asset mock, capturing the multipart fields of the matched request
func mockAddAsset(assetName string, status int, captured *capturedAsset) {
	gock.New(testURL).Post(assetJobPath).
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			if err := req.ParseMultipartForm(1 << 20); err != nil { //nolint:gosec // test request parsing
				return false, err
			}
			if req.MultipartForm.Value["asset_name"][0] != assetName {
				return false, nil
			}
			if captured != nil {
				*captured = capturedAsset{}
				for k, v := range req.MultipartForm.Value {
					(*captured)[k] = v[0]
				}
			}
			return true, nil
		}).
		Reply(status).JSON(assetJob("job-" + assetName))
}

func setup(t *testing.T) {
	t.Helper()
	gock.DisableNetworking()
	t.Cleanup(func() {
		gock.Off()
		gock.EnableNetworking()
	})
}

func TestSyncCreatesAppVersionAndAssets(t *testing.T) {
	setup(t)

	mockFindApp()
	gock.New(testURL).Post("/v2/apps").
		MatchType("json").
		JSON(map[string]interface{}{
			"name":        appName,
			"description": "Kubernetes Deployment default/web in cluster cluster1, managed by anchore-k8s-inventory",
			"contact":     map[string]interface{}{"name": "anchore-k8s-inventory"},
		}).
		Reply(201).JSON(appJSON(testAppID, appName))
	mockFindVersion()
	gock.New(testURL).Get(versionsPath).MatchParam("limit", "1000").Reply(200).JSON(list())
	gock.New(testURL).Post(versionsPath).
		MatchType("json").
		JSON(map[string]interface{}{
			"name":         versionName,
			"description":  "Deployment revision 2 (ReplicaSet web-abc123)",
			"status":       "released",
			"release_date": "2026-01-02T03:04:05Z",
		}).
		Reply(201).JSON(appJSON(testVersionID, versionName))
	mockListAssets()
	var nginx, sidecar capturedAsset
	mockAddAsset("nginx", 201, &nginx)
	mockAddAsset("sidecar", 201, &sidecar)

	s := NewSyncer()
	err := s.Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()})
	require.NoError(t, err)
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())

	assert.Equal(t, testVersionID, nginx["app_version_id"])
	assert.Equal(t, "container", nginx["asset_type"])
	assert.Equal(t, "docker.io/nginx:1.27@sha256:aaaa", nginx["image_reference"])
	annotations := map[string]string{}
	require.NoError(t, json.Unmarshal([]byte(nginx["annotations"]), &annotations))
	assert.Equal(t, map[string]string{
		"k8s.cluster":             "cluster1",
		"k8s.namespace":           "default",
		"k8s.deployment":          "web",
		"k8s.deployment.revision": "2",
		"k8s.replicaset":          "web-abc123",
		"k8s.container":           "nginx",
		"k8s.image_tag":           "docker.io/nginx:1.27",
		"k8s.image_digest":        "sha256:aaaa",
	}, annotations)
	assert.Equal(t, "docker.io/busybox:1.36", sidecar["image_reference"])

	// A second sync with no changes must not make any requests
	err = s.Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()})
	require.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestSyncExistingAppAndVersionAddsMissingAsset(t *testing.T) {
	setup(t)

	mockFindApp(appJSON(testAppID, appName))
	mockFindVersion(appJSON(testVersionID, versionName))
	mockListAssets("nginx")
	mockAddAsset("sidecar", 201, nil)

	err := NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()})
	require.NoError(t, err)
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
}

func TestSyncNewRevisionSetsPreviousVersion(t *testing.T) {
	setup(t)

	mockFindApp(appJSON(testAppID, appName))
	mockFindVersion()
	older := appJSON(testOlderID, "revision-0-old")
	older["system_metadata"] = map[string]interface{}{"id": testOlderID, "created_at": "2025-01-01T00:00:00Z"}
	newest := appJSON(testOldVerID, "revision-1-old")
	newest["system_metadata"] = map[string]interface{}{"id": testOldVerID, "created_at": "2025-06-01T00:00:00Z"}
	next := "next-page"
	gock.New(testURL).Get(versionsPath).MatchParam("limit", "1000").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			return req.URL.Query().Get("cursor") == "", nil
		}).
		Reply(200).JSON(map[string]interface{}{
		"pagination": map[string]interface{}{"item_count": 1, "next_cursor": next},
		"items":      []interface{}{older},
	})
	gock.New(testURL).Get(versionsPath).MatchParam("cursor", next).Reply(200).JSON(list(newest))
	gock.New(testURL).Post(versionsPath).
		MatchType("json").
		JSON(map[string]interface{}{
			"name":                versionName,
			"description":         "Deployment revision 2 (ReplicaSet web-abc123)",
			"status":              "released",
			"previous_version_id": testOldVerID,
			"release_date":        "2026-01-02T03:04:05Z",
		}).
		Reply(201).JSON(appJSON(testVersionID, versionName))
	mockListAssets("nginx", "sidecar")

	err := NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()})
	require.NoError(t, err)
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
}

func TestSyncAppCreateConflictRefinds(t *testing.T) {
	setup(t)

	mockFindApp()
	gock.New(testURL).Post("/v2/apps").Reply(409).JSON(map[string]interface{}{"detail": "exists", "status": 409})
	mockFindApp(appJSON(testAppID, appName))
	mockFindVersion(appJSON(testVersionID, versionName))
	mockListAssets("nginx", "sidecar")

	err := NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()})
	require.NoError(t, err)
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
}

func TestSyncAssetConflictIsSuccess(t *testing.T) {
	setup(t)

	mockFindApp(appJSON(testAppID, appName))
	mockFindVersion(appJSON(testVersionID, versionName))
	mockListAssets()
	mockAddAsset("nginx", 409, nil)
	mockAddAsset("sidecar", 201, nil)

	s := NewSyncer()
	err := s.Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()})
	require.NoError(t, err)
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())

	// Both assets are cached, no further requests
	require.NoError(t, s.Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()}))
}

func TestSyncAppsAPIUnsupported(t *testing.T) {
	setup(t)

	gock.New(testURL).Get("/v2/apps").Reply(404).JSON(map[string]interface{}{"detail": "not found", "status": 404})

	other := testDeployment()
	other.Name = "other"
	err := NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment(), other})
	assert.True(t, errors.Is(err, ErrAppsAPIUnsupported))
	assert.True(t, gock.IsDone())
}

func TestSyncForbidden(t *testing.T) {
	setup(t)

	gock.New(testURL).Get("/v2/apps").Reply(403).JSON(map[string]interface{}{"message": "Not authorized", "httpcode": 403})

	err := NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "createApplications")
}

func TestSyncErrorDoesNotStopOtherDeployments(t *testing.T) {
	setup(t)

	failing := testDeployment()
	failing.Name = "failing"
	gock.New(testURL).Get("/v2/apps").MatchParam("name", "^cluster1/default/failing$").
		Reply(500).JSON(map[string]interface{}{"message": "boom", "httpcode": 500})
	mockFindApp(appJSON(testAppID, appName))
	mockFindVersion(appJSON(testVersionID, versionName))
	mockListAssets("nginx", "sidecar")

	err := NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{failing, testDeployment()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default/failing")
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
}

func TestSyncApplicationsDisabled(t *testing.T) {
	setup(t)

	cfg := testConfig()
	cfg.CreateApplicationsFromDeployments = false
	assert.NoError(t, SyncApplications(cfg, "admin", []inventory.Deployment{testDeployment()}))
}

func TestNames(t *testing.T) {
	dep := testDeployment()
	assert.Equal(t, "cluster1/default/web", AppName("cluster1", dep))
	assert.Equal(t, "default/web", AppName("", dep))
	assert.Equal(t, "revision-2-abc123", VersionName(dep))
	dep.PodTemplateHash = ""
	assert.Equal(t, "revision-2", VersionName(dep))
}

func TestAssetAnnotationsOmitsEmpty(t *testing.T) {
	dep := testDeployment()
	got := assetAnnotations("", dep, dep.Containers[1])
	assert.NotContains(t, got, "k8s.cluster")
	assert.NotContains(t, got, "k8s.image_digest")
	assert.Equal(t, "sidecar", got["k8s.container"])
}
