package inventory

import (
	"context"
	"fmt"
	"regexp"
	"time"

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

func FetchNamespaces(c client.Client, batchSize, timeout int64, excludes, includes []string) ([]Namespace, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching namespaces")
	nsMap := make(map[string]Namespace)

	exclusionChecklist := buildExclusionChecklist(excludes)

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
		for _, n := range list.Items {
			if !excludeNamespace(exclusionChecklist, n.ObjectMeta.Name) {
				nsMap[n.ObjectMeta.Name] = Namespace{
					Name:        n.ObjectMeta.Name,
					UID:         string(n.UID),
					Annotations: n.Annotations,
					Labels:      n.Labels,
				}
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
			nsList = append(nsList, nsMap[ns])
		}
		return nsList, nil
	}

	// Return all namespaces (minus excludes) if no includes are set
	for _, ns := range nsMap {
		nsList = append(nsList, ns)
	}

	return nsList, nil
}
