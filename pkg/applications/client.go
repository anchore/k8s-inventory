package applications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
)

const (
	appsAPIPath                   = "v2/apps"
	appVersionsAPIPath            = "v2/apps/{{app_id}}/versions"
	appVersionAssetsAPIPath       = "v2/apps/{{app_id}}/versions/{{version_id}}/assets"
	addContainerImageAssetAPIPath = "v2/apps/{{app_id}}/jobs/add-container-image-asset"

	queryVersionID = "version_id"
	jobPending     = "pending"
	jobProcessing  = "processing"

	queryName             = "name"
	queryLimit            = "limit"
	queryCursor           = "cursor"
	pageLimit             = "1000"
	assetTypeContainer    = "container"
	versionStatusReleased = "released"
)

func appPath(path, appID string) string {
	return strings.Replace(path, "{{app_id}}", url.PathEscape(appID), 1)
}

func versionPath(path, appID, versionID string) string {
	return strings.Replace(appPath(path, appID), "{{version_id}}", url.PathEscape(versionID), 1)
}

func getJSON(details config.AnchoreInfo, path string, query url.Values, operation string, out interface{}) error {
	body, err := anchore.Get(path, query, details, operation)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(*body, out); err != nil {
		return fmt.Errorf("failed to parse %s response: %w", operation, err)
	}
	return nil
}

func postJSON(details config.AnchoreInfo, path string, req interface{}, operation string, out interface{}) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to serialize %s request: %w", operation, err)
	}
	body, err := anchore.Post(reqBody, "", path, details, operation)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(*body, out); err != nil {
		return fmt.Errorf("failed to parse %s response: %w", operation, err)
	}
	return nil
}

// findApp returns the app with the exact name, or nil if it does not exist
func findApp(details config.AnchoreInfo, name string) (*App, error) {
	resp := appListResponse{}
	query := url.Values{queryName: []string{name}, queryLimit: []string{pageLimit}}
	if err := getJSON(details, appsAPIPath, query, "app lookup", &resp); err != nil {
		return nil, err
	}
	for i := range resp.Items {
		if resp.Items[i].Name == name {
			return &resp.Items[i], nil
		}
	}
	return nil, nil
}

func createApp(details config.AnchoreInfo, req appCreateRequest) (*App, error) {
	app := App{}
	err := postJSON(details, appsAPIPath, req, "app create", &app)
	if anchore.IsHTTPStatus(err, http.StatusConflict) {
		// Created concurrently by someone else, use the existing app
		found, findErr := findApp(details, req.Name)
		if findErr != nil {
			return nil, findErr
		}
		if found == nil {
			return nil, fmt.Errorf("app %q reported as existing but could not be found: %w", req.Name, err)
		}
		log.Debugf("App %s already exists", req.Name)
		return found, nil
	}
	if err != nil {
		return nil, err
	}
	log.Infof("Created app %s", req.Name)
	return &app, nil
}

// findVersion returns the version of the app with the exact name, or nil if it does not exist
func findVersion(details config.AnchoreInfo, appID, name string) (*AppVersion, error) {
	resp := appVersionListResponse{}
	query := url.Values{queryName: []string{name}, queryLimit: []string{pageLimit}}
	if err := getJSON(details, appPath(appVersionsAPIPath, appID), query, "app version lookup", &resp); err != nil {
		return nil, err
	}
	for i := range resp.Items {
		if resp.Items[i].Name == name {
			return &resp.Items[i], nil
		}
	}
	return nil, nil
}

func listVersions(details config.AnchoreInfo, appID string) ([]AppVersion, error) {
	var versions []AppVersion
	query := url.Values{queryLimit: []string{pageLimit}}
	for {
		resp := appVersionListResponse{}
		if err := getJSON(details, appPath(appVersionsAPIPath, appID), query, "app version list", &resp); err != nil {
			return nil, err
		}
		versions = append(versions, resp.Items...)
		if resp.Pagination.NextCursor == nil || *resp.Pagination.NextCursor == "" {
			return versions, nil
		}
		query.Set(queryCursor, *resp.Pagination.NextCursor)
	}
}

func createVersion(details config.AnchoreInfo, appID string, req appVersionCreateRequest) (*AppVersion, error) {
	version := AppVersion{}
	err := postJSON(details, appPath(appVersionsAPIPath, appID), req, "app version create", &version)
	if anchore.IsHTTPStatus(err, http.StatusConflict) {
		found, findErr := findVersion(details, appID, req.Name)
		if findErr != nil {
			return nil, findErr
		}
		if found == nil {
			return nil, fmt.Errorf("app version %q reported as existing but could not be found: %w", req.Name, err)
		}
		log.Debugf("App version %s already exists", req.Name)
		return found, nil
	}
	if err != nil {
		return nil, err
	}
	log.Infof("Created app version %s", req.Name)
	return &version, nil
}

func listAssetNames(details config.AnchoreInfo, appID, versionID string) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	query := url.Values{queryLimit: []string{pageLimit}}
	for {
		resp := assetListResponse{}
		if err := getJSON(details, versionPath(appVersionAssetsAPIPath, appID, versionID), query, "app version asset list", &resp); err != nil {
			return nil, err
		}
		for _, a := range resp.Items {
			names[a.Name] = struct{}{}
		}
		if resp.Pagination.NextCursor == nil || *resp.Pagination.NextCursor == "" {
			return names, nil
		}
		query.Set(queryCursor, *resp.Pagination.NextCursor)
	}
}

// listInFlightAssetNames returns the names of assets with an add-container-image-asset job still pending or
// processing for the version, so a job is not submitted twice before the asset appears
func listInFlightAssetNames(details config.AnchoreInfo, appID, versionID string) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	query := url.Values{queryVersionID: []string{versionID}, queryLimit: []string{pageLimit}}
	for {
		resp := addContainerImageAssetJobListResponse{}
		if err := getJSON(details, appPath(addContainerImageAssetAPIPath, appID), query, "add container image asset job list", &resp); err != nil {
			return nil, err
		}
		for _, job := range resp.Items {
			if job.Status == jobPending || job.Status == jobProcessing {
				names[job.JobSpec.Asset.Name] = struct{}{}
			}
		}
		if resp.Pagination.NextCursor == nil || *resp.Pagination.NextCursor == "" {
			return names, nil
		}
		query.Set(queryCursor, *resp.Pagination.NextCursor)
	}
}

// addImageAsset submits an add-container-image-asset job. It returns the job ID, which is empty when an asset (or
// in-flight job) with the same name already exists on the version.
func addImageAsset(details config.AnchoreInfo, appID, versionID, name, ref string, annotations map[string]string) (string, error) {
	annotationsJSON, err := json.Marshal(annotations)
	if err != nil {
		return "", fmt.Errorf("failed to serialize asset annotations: %w", err)
	}
	fields := map[string]string{
		"app_version_id":  versionID,
		"asset_name":      name,
		"asset_type":      assetTypeContainer,
		"image_reference": ref,
		"annotations":     string(annotationsJSON),
	}
	body, err := anchore.PostMultipart(fields, appPath(addContainerImageAssetAPIPath, appID), details, "add container image asset")
	if anchore.IsHTTPStatus(err, http.StatusConflict) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	job := addContainerImageAssetJobResponse{}
	if err := json.Unmarshal(*body, &job); err != nil {
		return "", fmt.Errorf("failed to parse add container image asset response: %w", err)
	}
	return job.SystemMetadata.ID, nil
}
