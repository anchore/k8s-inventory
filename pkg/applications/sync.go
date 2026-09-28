/*
Package applications creates Anchore Apps and App Versions from Kubernetes Deployments. Each Deployment maps to an App
and each rollout (ReplicaSet revision) of the Deployment maps to an App Version, with the running container images
attached to the version as container assets.
*/
package applications

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/pkg/inventory"
)

const defaultContactName = "anchore-k8s-inventory"

// ErrAppsAPIUnsupported is returned when the Anchore Enterprise instance does not provide the Apps API
var ErrAppsAPIUnsupported = errors.New("anchore enterprise does not support the Apps API (requires Anchore Enterprise v6+)")

type cacheKey struct {
	account     string
	appName     string
	versionName string
}

type versionState struct {
	appID     string
	versionID string
	assets    map[string]struct{}
}

// Syncer reconciles Deployments with Anchore Apps. It caches what it knows to exist in Anchore so steady state polls
// make no requests.
type Syncer struct {
	mu    sync.Mutex
	cache map[cacheKey]*versionState
}

func NewSyncer() *Syncer {
	return &Syncer{cache: make(map[cacheKey]*versionState)}
}

// defaultSyncer keeps its cache across polls when running in periodic mode
var defaultSyncer = NewSyncer()

// SyncApplications ensures an Anchore App exists for each deployment, with an App Version for the current rollout
// containing the deployment's running container images as assets.
func SyncApplications(cfg *config.Application, account string, deployments []inventory.Deployment) error {
	if !cfg.CreateApplicationsFromDeployments || len(deployments) == 0 {
		return nil
	}
	details := cfg.AnchoreDetailsForAccount(account)
	if !details.IsValid() {
		log.Debug("Anchore details not specified, not creating applications")
		return nil
	}
	return defaultSyncer.Sync(cfg, details, deployments)
}

func AppName(cluster string, dep inventory.Deployment) string {
	if cluster == "" {
		return fmt.Sprintf("%s/%s", dep.Namespace, dep.Name)
	}
	return fmt.Sprintf("%s/%s/%s", cluster, dep.Namespace, dep.Name)
}

func VersionName(dep inventory.Deployment) string {
	if dep.PodTemplateHash == "" {
		return fmt.Sprintf("revision-%s", dep.Revision)
	}
	return fmt.Sprintf("revision-%s-%s", dep.Revision, dep.PodTemplateHash)
}

func imageReference(c inventory.Container) string {
	if c.ImageDigest != "" {
		return c.ImageTag + "@" + c.ImageDigest
	}
	return c.ImageTag
}

func assetAnnotations(cluster string, dep inventory.Deployment, c inventory.Container) map[string]string {
	all := map[string]string{
		"k8s.cluster":             cluster,
		"k8s.namespace":           dep.Namespace,
		"k8s.deployment":          dep.Name,
		"k8s.deployment.revision": dep.Revision,
		"k8s.replicaset":          dep.ReplicaSetName,
		"k8s.container":           c.Name,
		"k8s.image_tag":           c.ImageTag,
		"k8s.image_digest":        c.ImageDigest,
	}
	annotations := make(map[string]string, len(all))
	for k, v := range all {
		if v != "" {
			annotations[k] = v
		}
	}
	return annotations
}

// isFatal reports whether the error will affect every deployment, so syncing should stop for this account
func isFatal(err error) bool {
	return errors.Is(err, ErrAppsAPIUnsupported) || anchore.IsHTTPStatus(err, http.StatusForbidden)
}

// Sync reconciles the deployments with Anchore Apps for the account in details. Errors for one deployment do not
// prevent the others from being synced; they are joined and returned.
func (s *Syncer) Sync(cfg *config.Application, details config.AnchoreInfo, deployments []inventory.Deployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for _, dep := range deployments {
		err := s.syncDeployment(cfg, details, dep)
		if err == nil {
			continue
		}
		if isFatal(err) {
			if anchore.IsHTTPStatus(err, http.StatusForbidden) {
				err = fmt.Errorf("user %s is not authorized to manage applications in account %s "+
					"(requires readApplications, createApplications and createAssets permissions): %w", details.User, details.Account, err)
			}
			return err
		}
		errs = append(errs, fmt.Errorf("failed to sync application for deployment %s/%s: %w", dep.Namespace, dep.Name, err))
	}
	return errors.Join(errs...)
}

func (s *Syncer) syncDeployment(cfg *config.Application, details config.AnchoreInfo, dep inventory.Deployment) error {
	cluster := cfg.KubeConfig.Cluster
	appName := AppName(cluster, dep)
	versionName := VersionName(dep)
	key := cacheKey{account: details.Account, appName: appName, versionName: versionName}

	state, cached := s.cache[key]
	if !cached {
		var err error
		state, err = resolveVersion(cfg, details, dep, appName, versionName)
		if err != nil {
			return err
		}
	}

	var errs []error
	for _, c := range dep.Containers {
		if _, ok := state.assets[c.Name]; ok {
			continue
		}
		ref := imageReference(c)
		jobID, err := addImageAsset(details, state.appID, state.versionID, c.Name, ref, assetAnnotations(cluster, dep, c))
		if err != nil {
			if anchore.IsHTTPStatus(err, http.StatusNotFound) {
				// The version was removed from Anchore, resolve it again next time
				delete(s.cache, key)
				return err
			}
			if isFatal(err) {
				return err
			}
			errs = append(errs, fmt.Errorf("failed to add asset %s (%s): %w", c.Name, ref, err))
			continue
		}
		if jobID == "" {
			log.Debugf("Asset %s already exists or is being added (app %s version %s)", c.Name, appName, versionName)
		} else {
			log.Infof("Submitted add-container-image-asset job %s for %s (app %s version %s)", jobID, ref, appName, versionName)
		}
		state.assets[c.Name] = struct{}{}
	}
	s.cache[key] = state
	if len(errs) == 0 {
		log.Debugf("App %s version %s is up to date", appName, versionName)
	}
	return errors.Join(errs...)
}

// resolveVersion finds or creates the app and version for the deployment, returning the assets already present
func resolveVersion(cfg *config.Application, details config.AnchoreInfo, dep inventory.Deployment, appName, versionName string) (*versionState, error) {
	app, err := findOrCreateApp(cfg, details, dep, appName)
	if err != nil {
		return nil, err
	}
	appID := app.SystemMetadata.ID

	version, err := findVersion(details, appID, versionName)
	if err != nil {
		return nil, err
	}
	if version == nil {
		version, err = createNewVersion(details, appID, dep, versionName)
		if err != nil {
			return nil, err
		}
		log.Infof("Created app version %s for app %s", versionName, appName)
	}
	versionID := version.SystemMetadata.ID

	assets, err := listAssetNames(details, appID, versionID)
	if err != nil {
		return nil, err
	}
	return &versionState{appID: appID, versionID: versionID, assets: assets}, nil
}

func findOrCreateApp(cfg *config.Application, details config.AnchoreInfo, dep inventory.Deployment, appName string) (*App, error) {
	app, err := findApp(details, appName)
	if anchore.IsHTTPStatus(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrAppsAPIUnsupported, err)
	}
	if err != nil {
		return nil, err
	}
	if app != nil {
		return app, nil
	}

	contactName := cfg.Registration.IntegrationName
	if contactName == "" {
		contactName = defaultContactName
	}
	description := fmt.Sprintf("Kubernetes Deployment %s/%s", dep.Namespace, dep.Name)
	if cfg.KubeConfig.Cluster != "" {
		description += fmt.Sprintf(" in cluster %s", cfg.KubeConfig.Cluster)
	}
	description += ", managed by anchore-k8s-inventory"

	app, err = createApp(details, appCreateRequest{
		Name:        appName,
		Description: description,
		Contact:     contact{Name: contactName},
	})
	if err != nil {
		return nil, err
	}
	log.Infof("Created app %s", appName)
	return app, nil
}

func createNewVersion(details config.AnchoreInfo, appID string, dep inventory.Deployment, versionName string) (*AppVersion, error) {
	existing, err := listVersions(details, appID)
	if err != nil {
		return nil, err
	}
	var previousID *string
	var newest time.Time
	for i := range existing {
		if previousID == nil || existing[i].SystemMetadata.CreatedAt.After(newest) {
			previousID = &existing[i].SystemMetadata.ID
			newest = existing[i].SystemMetadata.CreatedAt
		}
	}

	req := appVersionCreateRequest{
		Name:              versionName,
		Description:       fmt.Sprintf("Deployment revision %s (ReplicaSet %s)", dep.Revision, dep.ReplicaSetName),
		Status:            versionStatusReleased,
		PreviousVersionID: previousID,
	}
	if !dep.ReplicaSetCreated.IsZero() {
		req.ReleaseDate = dep.ReplicaSetCreated.UTC().Format(time.RFC3339)
	}
	return createVersion(details, appID, req)
}
