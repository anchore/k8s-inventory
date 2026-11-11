/*
Package applications creates Anchore Apps and App Versions from Kubernetes workloads (Deployments, StatefulSets and
DaemonSets). Each workload maps to an App named <cluster>/<namespace>/<kind>/<name>, and each rollout of the workload
(a ReplicaSet revision for Deployments, a ControllerRevision for StatefulSets and DaemonSets) maps to an App Version,
with the running container images attached to the version as container assets.
*/
package applications

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
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
	// maxConsecutiveServerErrors is the number of workloads in a row failing with a server error after which the
	// sync for the account is stopped
	maxConsecutiveServerErrors = 3
	// maxAssetAnnotations is the most annotations the add-container-image-asset API accepts for an asset
	maxAssetAnnotations = 10
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

// Syncer reconciles workloads with Anchore Apps. It caches what it knows to exist in Anchore so steady state polls
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

// SyncApplications ensures an Anchore App exists for each workload, with an App Version for the current rollout
// containing the workload's running container images as assets.
func SyncApplications(cfg *config.Application, account string, workloads []inventory.Workload) error {
	if !cfg.CreatesApplications() || len(workloads) == 0 {
		return nil
	}
	details := cfg.AnchoreDetailsForAccount(account)
	if !details.IsValid() {
		log.Debug("Anchore details not specified, not creating applications")
		return nil
	}
	return defaultSyncer.Sync(cfg, details, workloads)
}

// kindName is the lower-case workload kind used in app names and logs (e.g. "statefulset")
func kindName(wl inventory.Workload) string {
	return strings.ToLower(string(wl.Kind))
}

// describe identifies the workload in logs and errors (e.g. "statefulset default/db")
func describe(wl inventory.Workload) string {
	return fmt.Sprintf("%s %s/%s", kindName(wl), wl.Namespace, wl.Name)
}

// AppName is <cluster>/<namespace>/<kind>/<name>. The kind keeps workloads of different kinds that share a name in a
// namespace apart.
func AppName(cluster string, wl inventory.Workload) string {
	if cluster == "" {
		return fmt.Sprintf("%s/%s/%s", wl.Namespace, kindName(wl), wl.Name)
	}
	return fmt.Sprintf("%s/%s/%s/%s", cluster, wl.Namespace, kindName(wl), wl.Name)
}

func VersionName(wl inventory.Workload) string {
	if wl.RevisionHash == "" {
		return fmt.Sprintf("revision-%s", wl.Revision)
	}
	return fmt.Sprintf("revision-%s-%s", wl.Revision, wl.RevisionHash)
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

// assetAnnotations returns at most maxAssetAnnotations non-empty annotations, in priority order. The workload's
// identity and its relations to other workloads come first; the lowest priority annotations repeat what the version
// (ReplicaSet or ControllerRevision name) and the asset itself (name, image reference) already record, so they are the
// ones left out when a workload has all of its relations set.
func assetAnnotations(cluster string, wl inventory.Workload, c inventory.Container) map[string]string {
	ordered := [][2]string{
		{"k8s.cluster", cluster},
		{"k8s.namespace", wl.Namespace},
		{"k8s.workload.kind", string(wl.Kind)},
		{"k8s.workload.name", wl.Name},
		{"k8s.workload.revision", wl.Revision},
		{"k8s.helm.release", wl.Relations.HelmRelease},
		{"k8s.app.instance", wl.Relations.Instance},
		{"k8s.app.part_of", wl.Relations.PartOf},
		{"k8s.app.component", wl.Relations.Component},
		{"k8s.owner", owner(wl)},
		{"k8s." + strings.ToLower(wl.RevisionKind()), wl.RevisionName},
		{"k8s.container", c.Name},
		{"k8s.image_tag", c.ImageTag},
		{"k8s.image_digest", c.ImageDigest},
	}
	annotations := make(map[string]string, maxAssetAnnotations)
	for _, kv := range ordered {
		if kv[1] == "" {
			continue
		}
		if len(annotations) == maxAssetAnnotations {
			break
		}
		annotations[kv[0]] = kv[1]
	}
	return annotations
}

// owner is the workload's controlling owner as <Kind>/<name> (e.g. an operator custom resource), or empty
func owner(wl inventory.Workload) string {
	if wl.Relations.OwnerKind == "" {
		return ""
	}
	return wl.Relations.OwnerKind + "/" + wl.Relations.OwnerName
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

// isFatal reports whether the error will affect every workload, so syncing should stop for this account rather
// than making (and possibly waiting out timeouts for) further requests
func isFatal(err error) bool {
	return errors.Is(err, ErrAppsAPIUnsupported) || anchore.IsHTTPStatus(err, http.StatusForbidden) || isUnavailable(err)
}

// Sync reconciles the workloads with Anchore Apps for the account in details. Errors for one workload do not
// prevent the others from being synced; they are joined and returned. Errors that would affect every workload
// (Apps API unsupported, forbidden, Anchore unavailable) stop the sync for the account immediately.
func (s *Syncer) Sync(cfg *config.Application, details config.AnchoreInfo, workloads []inventory.Workload) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	desired := make(map[cacheKey]struct{}, len(workloads))
	var errs []error
	consecutiveServerErrors := 0
	for _, wl := range workloads {
		desired[s.key(cfg, details, wl)] = struct{}{}
		err := s.syncWorkload(cfg, details, wl)
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
			errs = append(errs, fmt.Errorf("failed to sync application for %s: %w", describe(wl), err))
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
		errs = append(errs, fmt.Errorf("failed to sync application for %s: %w", describe(wl), err))
	}
	s.prune(details.Account, desired)
	return errors.Join(errs...)
}

func (s *Syncer) key(cfg *config.Application, details config.AnchoreInfo, wl inventory.Workload) cacheKey {
	return cacheKey{account: details.Account, appName: AppName(cfg.KubeConfig.Cluster, wl), versionName: VersionName(wl)}
}

// prune removes cached versions for the account that are no longer the current rollout of a workload, so the
// cache does not grow without bound as workloads are rolled out
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

func (s *Syncer) syncWorkload(cfg *config.Application, details config.AnchoreInfo, wl inventory.Workload) error {
	cluster := cfg.KubeConfig.Cluster
	key := s.key(cfg, details, wl)
	appName, versionName := key.appName, key.versionName

	// Only containers with a known image digest and a valid reference are added, so the asset references exactly
	// what is running and no version is created for a workload whose images cannot be added.
	// Containers without a digest yet (e.g. still pulling) are retried on a later poll.
	var containers []assetContainer
	for _, c := range wl.Containers {
		if c.ImageDigest == "" {
			log.Debugf("Container %s of %s has no image digest yet, will retry on a later poll", c.Name, describe(wl))
			continue
		}
		ref, err := imageReference(c)
		if err != nil {
			log.Warnf("Not adding container %s of %s as an application asset: %v", c.Name, describe(wl), err)
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
		state, err = resolveVersion(cfg, details, wl, appName, versionName)
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
		jobID, err := addImageAsset(details, state.appID, state.versionID, c.Name, ref, assetAnnotations(cluster, wl, c.Container))
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

// resolveVersion finds or creates the app and version for the workload, returning the assets already present
func resolveVersion(cfg *config.Application, details config.AnchoreInfo, wl inventory.Workload, appName, versionName string) (*versionState, error) {
	app, err := findOrCreateApp(cfg, details, wl, appName)
	if err != nil {
		return nil, err
	}
	appID := app.SystemMetadata.ID

	version, err := findVersion(details, appID, versionName)
	if err != nil {
		return nil, err
	}
	if version == nil {
		version, err = createNewVersion(details, appID, wl, versionName)
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

func findOrCreateApp(cfg *config.Application, details config.AnchoreInfo, wl inventory.Workload, appName string) (*App, error) {
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
	app, err = createApp(details, appCreateRequest{
		Name:        appName,
		Description: appDescription(cfg.KubeConfig.Cluster, wl),
		Contact:     contact{Name: contactName},
	})
	if err != nil {
		return nil, err
	}
	return app, nil
}

// appDescription names the workload and what relates it to other workloads, e.g.
// "Kubernetes StatefulSet harbor/harbor-database (Helm release harbor, part of harbor) in cluster lab, managed by
// anchore-k8s-inventory"
func appDescription(cluster string, wl inventory.Workload) string {
	description := fmt.Sprintf("Kubernetes %s %s/%s", wl.Kind, wl.Namespace, wl.Name)
	var related []string
	r := wl.Relations
	switch {
	case r.HelmRelease != "":
		related = append(related, "Helm release "+r.HelmRelease)
	case r.Instance != "":
		related = append(related, "instance "+r.Instance)
	}
	if r.PartOf != "" {
		related = append(related, "part of "+r.PartOf)
	}
	if o := owner(wl); o != "" {
		related = append(related, "owned by "+o)
	}
	if len(related) > 0 {
		description += " (" + strings.Join(related, ", ") + ")"
	}
	if cluster != "" {
		description += fmt.Sprintf(" in cluster %s", cluster)
	}
	return description + ", managed by anchore-k8s-inventory"
}

func createNewVersion(details config.AnchoreInfo, appID string, wl inventory.Workload, versionName string) (*AppVersion, error) {
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
		Description:       fmt.Sprintf("%s revision %s (%s %s)", wl.Kind, wl.Revision, wl.RevisionKind(), wl.RevisionName),
		Status:            versionStatusReleased,
		PreviousVersionID: previousID,
	}
	if !wl.RevisionCreated.IsZero() {
		req.ReleaseDate = wl.RevisionCreated.UTC().Format(time.RFC3339)
	}
	return createVersion(details, appID, req)
}
