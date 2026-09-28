package applications

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
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

var (
	released    = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	nginxDigest = "sha256:" + strings.Repeat("a", 64)
	bbDigest    = "sha256:" + strings.Repeat("b", 64)
)

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
			{Name: "nginx", ImageTag: "docker.io/nginx:1.27", ImageDigest: nginxDigest},
			{Name: "sidecar", ImageTag: "docker.io/busybox:1.36", ImageDigest: bbDigest},
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

// mockListAssets mocks the asset list for the version, and an empty list of in-flight asset jobs
func mockListAssets(names ...string) {
	items := make([]interface{}, 0, len(names))
	for _, n := range names {
		items = append(items, map[string]interface{}{"name": n, "type": "container"})
	}
	gock.New(testURL).Get(assetsPath).Reply(200).JSON(list(items...))
	mockListJobs()
}

func mockListJobs(jobs ...interface{}) {
	gock.New(testURL).Get(assetJobPath).MatchParam("version_id", testVersionID).Reply(200).JSON(list(jobs...))
}

func job(assetName, status string) map[string]interface{} {
	return map[string]interface{}{
		"status":   status,
		"job_type": "add-container-image-asset",
		"job_spec": map[string]interface{}{
			"asset":           map[string]interface{}{"name": assetName, "type": "container"},
			"image_reference": "docker.io/library/" + assetName,
		},
	}
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
	assert.Equal(t, "docker.io/library/nginx:1.27@"+nginxDigest, nginx["image_reference"])
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
		"k8s.image_digest":        nginxDigest,
	}, annotations)
	assert.Equal(t, "docker.io/library/busybox:1.36@"+bbDigest, sidecar["image_reference"])

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
		Reply(400).JSON(map[string]interface{}{"message": "bad request", "httpcode": 400})
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
	dep.Containers[1].ImageDigest = ""
	got := assetAnnotations("", dep, dep.Containers[1])
	assert.NotContains(t, got, "k8s.cluster")
	assert.NotContains(t, got, "k8s.image_digest")
	assert.Equal(t, "sidecar", got["k8s.container"])
}

func TestSyncStopsWhenAppsAPIUnavailable(t *testing.T) {
	tests := []struct {
		name string
		mock func(*gock.Response)
	}{
		{
			name: "service unavailable",
			mock: func(r *gock.Response) {
				r.Status(503).JSON(map[string]interface{}{"message": "unavailable", "httpcode": 503})
			},
		},
		{
			name: "timeout",
			mock: func(r *gock.Response) {
				r.SetError(&timeoutError{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			failing := testDeployment()
			failing.Name = "failing"
			tt.mock(gock.New(testURL).Get("/v2/apps").MatchParam("name", "^cluster1/default/failing$").Reply(200))

			// No mocks are registered for the second deployment: any request for it fails the test via the
			// pending/unmatched checks below
			err := NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{failing, testDeployment()})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unavailable")
			assert.True(t, gock.IsDone())
			assert.False(t, gock.HasUnmatchedRequest(), "unexpected requests: %v", gock.GetUnmatchedRequests())
		})
	}
}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestSyncSkipsContainersWithoutDigest(t *testing.T) {
	setup(t)

	dep := testDeployment()
	dep.Containers[1].ImageDigest = ""

	mockFindApp(appJSON(testAppID, appName))
	mockFindVersion(appJSON(testVersionID, versionName))
	mockListAssets("nginx")

	s := NewSyncer()
	require.NoError(t, s.Sync(testConfig(), testDetails(), []inventory.Deployment{dep}))
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
	assert.False(t, gock.HasUnmatchedRequest())

	// Once the digest is known the asset is added, using the cached app and version
	dep.Containers[1].ImageDigest = bbDigest
	var sidecar capturedAsset
	mockAddAsset("sidecar", 201, &sidecar)
	require.NoError(t, s.Sync(testConfig(), testDetails(), []inventory.Deployment{dep}))
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
	assert.Equal(t, "docker.io/library/busybox:1.36@"+bbDigest, sidecar["image_reference"])
}

func TestSyncSkipsDeploymentWithNoDigests(t *testing.T) {
	setup(t)

	dep := testDeployment()
	for i := range dep.Containers {
		dep.Containers[i].ImageDigest = ""
	}
	require.NoError(t, NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{dep}))
	assert.False(t, gock.HasUnmatchedRequest())
}

func TestSyncPrunesCache(t *testing.T) {
	setup(t)

	s := NewSyncer()
	current := s.key(testConfig(), testDetails(), testDeployment())
	stale := cacheKey{account: "admin", appName: appName, versionName: "revision-1-old"}
	otherAccount := cacheKey{account: "other", appName: appName, versionName: "revision-1-old"}
	for _, k := range []cacheKey{current, stale, otherAccount} {
		s.cache[k] = &versionState{appID: testAppID, versionID: testVersionID,
			assets: map[string]struct{}{"nginx": {}, "sidecar": {}}}
	}

	require.NoError(t, s.Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()}))
	assert.Contains(t, s.cache, current)
	assert.NotContains(t, s.cache, stale)
	assert.Contains(t, s.cache, otherAccount)
}

func TestSyncSingleServerErrorDoesNotStopOtherDeployments(t *testing.T) {
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
	assert.NotContains(t, err.Error(), "unavailable")
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
}

func TestSyncStopsAfterConsecutiveServerErrors(t *testing.T) {
	setup(t)

	var deps []inventory.Deployment
	for _, name := range []string{"a", "b", "c", "d"} {
		dep := testDeployment()
		dep.Name = name
		deps = append(deps, dep)
	}
	for _, name := range []string{"a", "b", "c"} {
		gock.New(testURL).Get("/v2/apps").MatchParam("name", "^cluster1/default/"+name+"$").
			Reply(500).JSON(map[string]interface{}{"message": "boom", "httpcode": 500})
	}

	// No mock for deployment d: a request for it would be unmatched
	err := NewSyncer().Sync(testConfig(), testDetails(), deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 consecutive server errors")
	assert.True(t, gock.IsDone())
	assert.False(t, gock.HasUnmatchedRequest(), "unexpected requests: %v", gock.GetUnmatchedRequests())
}

func TestSyncSkipsAssetsWithInFlightJobs(t *testing.T) {
	setup(t)

	mockFindApp(appJSON(testAppID, appName))
	mockFindVersion(appJSON(testVersionID, versionName))
	gock.New(testURL).Get(assetsPath).Reply(200).JSON(list())
	// nginx is still being added, a failed sidecar job must be retried
	mockListJobs(job("nginx", "processing"), job("sidecar", "failed"))
	mockAddAsset("sidecar", 201, nil)

	require.NoError(t, NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{testDeployment()}))
	assert.True(t, gock.IsDone(), "pending mocks: %v", gock.Pending())
	assert.False(t, gock.HasUnmatchedRequest(), "unexpected requests: %v", gock.GetUnmatchedRequests())
}

func TestSyncSkipsInvalidImageReferences(t *testing.T) {
	setup(t)

	dep := testDeployment()
	dep.Containers[0].ImageTag = "Invalid/UPPER:tag"
	dep.Containers[1].ImageDigest = "sha256:short"

	// No version is created when none of the containers can be added
	require.NoError(t, NewSyncer().Sync(testConfig(), testDetails(), []inventory.Deployment{dep}))
	assert.False(t, gock.HasUnmatchedRequest(), "unexpected requests: %v", gock.GetUnmatchedRequests())
}

func TestImageReference(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		digest  string
		want    string
		wantErr bool
	}{
		{name: "short docker hub name", tag: "nginx:1.25-alpine", digest: nginxDigest, want: "docker.io/library/nginx:1.25-alpine@" + nginxDigest},
		{name: "docker hub org", tag: "bitnami/redis:7", digest: nginxDigest, want: "docker.io/bitnami/redis:7@" + nginxDigest},
		{name: "docker hub without library", tag: "docker.io/busybox:1.36", digest: bbDigest, want: "docker.io/library/busybox:1.36@" + bbDigest},
		{name: "other registry", tag: "quay.io/org/app:v1", digest: bbDigest, want: "quay.io/org/app:v1@" + bbDigest},
		{name: "registry with port", tag: "localhost:5000/app:v1", digest: bbDigest, want: "localhost:5000/app:v1@" + bbDigest},
		{name: "no tag", tag: "nginx", digest: nginxDigest, want: "docker.io/library/nginx:latest@" + nginxDigest},
		{name: "no digest", tag: "nginx:1.25", want: "docker.io/library/nginx:1.25"},
		{name: "invalid name", tag: "Invalid/UPPER:tag", digest: nginxDigest, wantErr: true},
		{name: "invalid digest", tag: "nginx:1.25", digest: "sha256:short", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := imageReference(inventory.Container{ImageTag: tt.tag, ImageDigest: tt.digest})
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
