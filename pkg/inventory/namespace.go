package inventory

import (
	"context"
	"fmt"
	"regexp"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/client"
)

// excludeCheck is a function that will return whether a namespace should be
// excluded based on a regex or direct string match
type excludeCheck func(namespace string) bool

// excludeRegex compiles a regex to use for namespace matching
func excludeRegex(check string) excludeCheck {
	return func(namespace string) bool {
		return regexp.MustCompile(check).MatchString(namespace)
	}
}

// excludeSet checks if a given string is present is a set
func excludeSet(check map[string]struct{}) excludeCheck {
	return func(namespace string) bool {
		_, exist := check[namespace]
		return exist
	}
}

// Regex to determine whether a string is a valid namespace (valid dns name)
var validNamespaceRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// buildExclusionChecklist will create a list of checks based on the configured
// exclusion strings. The checks could be regexes or direct string matches.
// It will create a regex check if the namespace string is not a valid dns
// name. If the namespace string in the exclude list is a valid dns name then
// it will add it to a map for direct lookup when the checks are run.
func buildExclusionChecklist(exclusions []string) []excludeCheck {
	var excludeChecks []excludeCheck

	if len(exclusions) > 0 {
		excludeMap := make(map[string]struct{})

		for _, ex := range exclusions {
			if !validNamespaceRegex.MatchString(ex) {
				// assume the check is a regex
				excludeChecks = append(excludeChecks, excludeRegex(ex))
			} else {
				// assume check is raw string so add to set for lookup
				excludeMap[ex] = struct{}{}
			}
		}
		excludeChecks = append(excludeChecks, excludeSet(excludeMap))
	}

	return excludeChecks
}

// excludeNamespace is a helper function to check whether a namespace matches
// any of the exclusion rules
func excludeNamespace(checks []excludeCheck, namespace string) bool {
	for _, check := range checks {
		if check(namespace) {
			return true
		}
	}
	return false
}

// NamespaceFilter holds the compiled include/exclude rules for namespaces so
// that they can be built once and reused for every namespace that needs
// checking (e.g. for each event received from an informer).
type NamespaceFilter struct {
	excludeChecks []excludeCheck
	includes      map[string]struct{}
}

// NewNamespaceFilter builds a reusable filter from the configured namespace
// selectors. When includes is non-empty it acts as an allow-list, otherwise
// every namespace that is not excluded is kept.
func NewNamespaceFilter(excludes, includes []string) NamespaceFilter {
	includeSet := make(map[string]struct{}, len(includes))
	for _, ns := range includes {
		includeSet[ns] = struct{}{}
	}

	return NamespaceFilter{
		excludeChecks: buildExclusionChecklist(excludes),
		includes:      includeSet,
	}
}

// Keep reports whether the named namespace should be part of the inventory
func (f NamespaceFilter) Keep(namespace string) bool {
	if excludeNamespace(f.excludeChecks, namespace) {
		return false
	}
	if len(f.includes) > 0 {
		_, included := f.includes[namespace]
		return included
	}
	return true
}

// NamespaceFromV1 converts a kubernetes namespace into its inventory
// representation, applying the configured metadata collection rules
func NamespaceFromV1(n *v1.Namespace, includeAnnotations, includeLabels []string, disableMetadata bool) Namespace {
	ns := Namespace{
		Name: n.Name,
		UID:  string(n.UID),
	}
	if !disableMetadata {
		ns.Annotations = processAnnotationsOrLabels(n.Annotations, includeAnnotations)
		ns.Labels = processAnnotationsOrLabels(n.Labels, includeLabels)
	}
	return ns
}

func FetchNamespaces(
	c client.Client,
	batchSize, timeout int64,
	excludes, includes []string,
	includeAnnotations, includeLabels []string,
	disableMetadata bool,
) ([]Namespace, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching namespaces")
	nsMap := make(map[string]Namespace)

	filter := NewNamespaceFilter(excludes, nil)

	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          batchSize,
			Continue:       cont,
			TimeoutSeconds: &timeout,
		}

		list, err := c.Clientset.CoreV1().Namespaces().List(context.Background(), opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list namespaces: %w", err)
		}
		for i := range list.Items {
			n := &list.Items[i]
			if filter.Keep(n.Name) {
				nsMap[n.Name] = NamespaceFromV1(n, includeAnnotations, includeLabels, disableMetadata)
			}
		}

		cont = list.GetListMeta().GetContinue()
		if cont == "" {
			break
		}
	}

	var nsList []Namespace

	// Only return namespaces that are explicitly included if set
	if len(includes) > 0 {
		for _, ns := range includes {
			if _, exists := nsMap[ns]; exists {
				nsList = append(nsList, nsMap[ns])
			}
		}
		return nsList, nil
	}

	// Return all namespaces (minus excludes) if no includes are set
	for _, ns := range nsMap {
		nsList = append(nsList, ns)
	}

	return nsList, nil
}
