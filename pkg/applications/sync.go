/*
Package applications creates Anchore Apps and App Versions from Kubernetes Deployments. Each Deployment maps to an App
and each rollout (ReplicaSet revision) of the Deployment maps to an App Version, with the running container images
attached to the version as container assets.
*/
package applications

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/distribution/reference"

	"github.com/anchore/k8s-inventory/internal/anchore"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/pkg/inventory"
)

const (
	defaultContactName = "anchore-k8s-inventory"
	// maxImageReferenceLength is the longest image_reference accepted by the add-container-image-asset API
	maxImageReferenceLength = 512
	// maxConsecutiveServerErrors is the number of deployments in a row failing with a server error after which the
	// sync for the account is stopped
	maxConsecutiveServerErrors = 3
)

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

// imageReference returns the fully qualified, digest pinned reference for the container image, as required by the
// add-container-image-asset API (e.g. nginx:1.27 -> docker.io/library/nginx:1.27@sha256:...)
func imageReference(c inventory.Container) (string, error) {
	named, err := reference.ParseNormalizedNamed(c.ImageTag)
	if err != nil {
		return "", fmt.Errorf("invalid image reference %q: %w", c.ImageTag, err)
	}
	ref := reference.TagNameOnly(named).String()
	if c.ImageDigest != "" {
		ref += "@" + c.ImageDigest
		if _, err := reference.ParseNormalizedNamed(ref); err != nil {
			return "", fmt.Errorf("invalid image reference %q: %w", ref, err)
		}
	}
	if len(ref) > maxImageReferenceLength {
		return "", fmt.Errorf("image reference %q is longer than %d characters", ref, maxImageReferenceLength)
	}
	return ref, nil
}

// assetContainer is a container to add as an asset, with its image reference
type assetContainer struct {
	inventory.Container
	ref string
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

// isUnavailable reports whether Anchore (or its Apps API backend) is unreachable, timing out or unavailable
// (502/503/504), in which case every further request is expected to fail as well
func isUnavailable(err error) bool {
	if anchore.ServerIsOffline(err) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// isServerError reports whether the error is a 5xx response. A single server error may be specific to one request
// (e.g. one image), so only repeated server errors stop the sync.
func isServerError(err error) bool {
	var apiErr *anchore.APIClientError
	return errors.As(err, &apiErr) && apiErr.HTTPStatusCode >= 500 && apiErr.HTTPStatusCode <= 599
}

// isFatal reports whether the error will affect every deployment, so syncing should stop for this account rather
// than making (and possibly waiting out timeouts for) further requests
func isFatal(err error) bool {
	return errors.Is(err, ErrAppsAPIUnsupported) || anchore.IsHTTPStatus(err, http.StatusForbidden) || isUnavailable(err)
}

// Sync reconciles the deployments with Anchore Apps for the account in details. Errors for one deployment do not
// prevent the others from being synced; they are joined and returned. Errors that would affect every deployment
// (Apps API unsupported, forbidden, Anchore unavailable) stop the sync for the account immediately.
func (s *Syncer) Sync(cfg *config.Application, details config.AnchoreInfo, deployments []inventory.Deployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	desired := make(map[cacheKey]struct{}, len(deployments))
	var errs []error
	consecutiveServerErrors := 0
	for _, dep := range deployments {
		desired[s.key(cfg, details, dep)] = struct{}{}
		err := s.syncDeployment(cfg, details, dep)
		if err == nil {
			consecutiveServerErrors = 0
			continue
		}
		if isServerError(err) {
			consecutiveServerErrors++
		} else {
			consecutiveServerErrors = 0
		}
		if consecutiveServerErrors >= maxConsecutiveServerErrors {
			errs = append(errs, fmt.Errorf("failed to sync application for deployment %s/%s: %w", dep.Namespace, dep.Name, err))
			return fmt.Errorf("anchore apps API unavailable (%d consecutive server errors), skipping remaining application "+
				"sync for account %s: %w", consecutiveServerErrors, details.Account, errors.Join(errs...))
		}
		if isFatal(err) {
			switch {
			case anchore.IsHTTPStatus(err, http.StatusForbidden):
				err = fmt.Errorf("user %s is not authorized to manage applications in account %s "+
					"(requires readApplications, createApplications and createAssets permissions): %w", details.User, details.Account, err)
			case isUnavailable(err):
				err = fmt.Errorf("anchore apps API unavailable, skipping remaining application sync for account %s: %w", details.Account, err)
			}
			return err
		}
		errs = append(errs, fmt.Errorf("failed to sync application for deployment %s/%s: %w", dep.Namespace, dep.Name, err))
	}
	s.prune(details.Account, desired)
	return errors.Join(errs...)
}

func (s *Syncer) key(cfg *config.Application, details config.AnchoreInfo, dep inventory.Deployment) cacheKey {
	return cacheKey{account: details.Account, appName: AppName(cfg.KubeConfig.Cluster, dep), versionName: VersionName(dep)}
}

// prune removes cached versions for the account that are no longer the current rollout of a deployment, so the
// cache does not grow without bound as deployments are rolled out
func (s *Syncer) prune(account string, desired map[cacheKey]struct{}) {
	for key := range s.cache {
		if key.account != account {
			continue
		}
		if _, ok := desired[key]; !ok {
			delete(s.cache, key)
		}
	}
}

func (s *Syncer) syncDeployment(cfg *config.Application, details config.AnchoreInfo, dep inventory.Deployment) error {
	cluster := cfg.KubeConfig.Cluster
	key := s.key(cfg, details, dep)
	appName, versionName := key.appName, key.versionName

	// Only containers with a known image digest and a valid reference are added, so the asset references exactly
	// what is running and no version is created for a deployment whose images cannot be added.
	// Containers without a digest yet (e.g. still pulling) are retried on a later poll.
	var containers []assetContainer
	for _, c := range dep.Containers {
		if c.ImageDigest == "" {
			log.Debugf("Container %s of deployment %s/%s has no image digest yet, will retry on a later poll", c.Name, dep.Namespace, dep.Name)
			continue
		}
		ref, err := imageReference(c)
		if err != nil {
			log.Warnf("Not adding container %s of deployment %s/%s as an application asset: %v", c.Name, dep.Namespace, dep.Name, err)
			continue
		}
		containers = append(containers, assetContainer{Container: c, ref: ref})
	}
	if len(containers) == 0 {
		return nil
	}

	state, cached := s.cache[key]
	if !cached {
		var err error
		state, err = resolveVersion(cfg, details, dep, appName, versionName)
		if err != nil {
			return err
		}
	}

	var errs []error
	for _, c := range containers {
		if _, ok := state.assets[c.Name]; ok {
			continue
		}
		ref := c.ref
		jobID, err := addImageAsset(details, state.appID, state.versionID, c.Name, ref, assetAnnotations(cluster, dep, c.Container))
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
	}
	versionID := version.SystemMetadata.ID

	assets, err := listAssetNames(details, appID, versionID)
	if err != nil {
		return nil, err
	}
	inFlight, err := listInFlightAssetNames(details, appID, versionID)
	if err != nil {
		return nil, err
	}
	for name := range inFlight {
		assets[name] = struct{}{}
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
