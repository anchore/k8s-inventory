package inventory

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anchore/kai/internal/log"
	v1 "k8s.io/api/core/v1"
)

// ReportItem represents a namespace and all it's unique images
type ReportItem struct {
	Namespace string        `json:"namespace,omitempty"`
	Images    []ReportImage `json:"images"`
}

// ReportImage represents a unique image in a cluster
type ReportImage struct {
	Tag        string `json:"tag,omitempty"`
	RepoDigest string `json:"repoDigest,omitempty"`
}

// NewReportItem parses a list of pods into a ReportItem full of unique images
func NewReportItem(pods []v1.Pod, namespace string, ignoreNotRunning bool, missingTagPolicy string, dummyTag string) ReportItem {
	reportItem := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}

	for _, pod := range pods {
		// Check for non-running
		if ignoreNotRunning && pod.Status.Phase != "Running" {
			continue
		}
		reportItem.extractUniqueImages(pod, missingTagPolicy, dummyTag)
	}

	return reportItem
}

// String represent the ReportItem as a string
func (r *ReportItem) String() string {
	return fmt.Sprintf("ReportItem(namespace=%s, images=%v)", r.Namespace, r.Images)
}

// key will return a unique key for a ReportImage
func (i *ReportImage) key() string {
	return fmt.Sprintf("%s@%s", i.Tag, i.RepoDigest)
}

// Adds an ReportImage to the ReportItem struct (if it doesn't exist there already)
//
// IMPORTANT: Ensures unique images across pods
func (r *ReportItem) extractUniqueImages(pod v1.Pod, missingTagPolicy string, dummyTag string) {
	// Build a Map to make use as a Set (unique list). Values
	// are empty structs so they don't waste space
	unique := make(map[string]struct{})
	for _, image := range r.Images {
		unique[image.key()] = struct{}{}
	}

	// Process all containers in a pod and return all the unique images
	images := processContainers(pod, missingTagPolicy, dummyTag)

	// If the image isn't in the set already, append it to the list
	for _, image := range images {
		if _, exist := unique[image.key()]; !exist {
			r.Images = append(r.Images, image)
		}
	}
}

// fillContainerDetails grabs all the relevant fields out of a pod object so
// they can be used to parse out the image details for all the containers in
// a pod. Return details as an mapped array of strings using the container name
// as the map key and the fields as an array of strings so they can be iterated
func fillContainerDetails(pod v1.Pod) map[string][]string {
	details := make(map[string][]string)

	// grab init images
	for _, c := range pod.Spec.InitContainers {
		details[c.Name] = append(details[c.Name], c.Image)
	}

	for _, c := range pod.Status.InitContainerStatuses {
		details[c.Name] = append(details[c.Name], c.Image, c.ImageID)
	}

	// grab regular images
	for _, c := range pod.Spec.Containers {
		details[c.Name] = append(details[c.Name], c.Image)
	}

	for _, c := range pod.Status.ContainerStatuses {
		details[c.Name] = append(details[c.Name], c.Image, c.ImageID)
	}

	return details
}

// image is an intermediate struct for parsing out image details from
// a list of containers
type image struct {
	repo   string
	tag    string
	digest string
}

// Compile the regexes used for parsing once so they can be reused without having to recompile
var digestRegex = regexp.MustCompile(`@(sha[[:digit:]]{3}:[[:alnum:]]{32,})`)
var tagRegex = regexp.MustCompile(`:[\w][\w.-]{0,127}$`)

// extractImageDetails extracts the repo, tag, and digest of an image out of the fields
// grabbed from the pod.
func (img *image) extractImageDetails(s string) {
	if img.digest != "" && img.tag != "" && img.repo != "" {
		return
	}

	// Attempt to grab the digest out of the string
	// Set repo to the initial string. If there's no digest to parse then we can assume
	// it's just a repo and tag
	repo := s
	digest := ""

	// Look for something like:
	//  k3d-registry.localhost:5000/redis:4@sha256:5bd4fe08813b057df2ae55003a75c39d80a4aea9f1a0fbc0fbd7024edf555786
	digestresult := digestRegex.FindStringSubmatchIndex(repo)
	if len(digestresult) > 0 {
		i := digestresult[0]
		digest = repo[i+1:] // sha256:5bd4fe08813b057df2ae55003a75c39d80a4aea9f1a0fbc0fbd7024edf555786
		repo = repo[:i]     // k3d-registry.localhost:5000/redis:4
	}

	// Attempt to split the repo and tag
	tag := ""

	// repo contains something like
	//  k3d-registry.localhost:5000/redis:4
	tagresult := tagRegex.FindStringSubmatchIndex(repo)
	if len(tagresult) > 0 {
		i := tagresult[0]
		tag = repo[i+1:] // 4
		repo = repo[:i]  // k3d-registry.localhost:5000/redis
	}

	// Only fill if the field hasn't been successfully parsed yet
	if img.digest == "" {
		img.digest = digest
	}

	if img.tag == "" {
		img.tag = tag
	}

	if img.repo == "" {
		img.repo = repo
	}
}

func (img *image) handleMissingTag(missingTagPolicy string, dummyTag string) {
	switch missingTagPolicy {
	case "digest":
		tag := strings.Split(img.digest, ":")
		img.tag = tag[len(tag)-1]
	case "insert":
		img.tag = dummyTag
	}
}

// processContainers takes in a pod object and will return a list of unique
// ReportImage structures from the containers inside the pod
//
// IMPORTANT: Ensures unique images inside a pod
func processContainers(pod v1.Pod, missingTagPolicy string, dummyTag string) []ReportImage {
	unique := make(map[string]image)

	containerset := fillContainerDetails(pod)
	for _, containerdata := range containerset {
		img := image{
			repo:   "",
			tag:    "",
			digest: "",
		}

		for _, container := range containerdata {
			img.extractImageDetails(container)
		}

		if img.tag == "" {
			if missingTagPolicy == "drop" {
				log.Debugf("Dropping %s %s due to missing tag policy of 'drop'", img.repo, img.digest)
				continue
			}
			img.handleMissingTag(missingTagPolicy, dummyTag)
		}

		key := fmt.Sprintf("%s:%s@%s", img.repo, img.tag, img.digest)
		unique[key] = img
	}

	ri := make([]ReportImage, 0)
	for _, u := range unique {
		ri = append(ri, ReportImage{
			Tag:        fmt.Sprintf("%s:%s", u.repo, u.tag),
			RepoDigest: u.digest,
		})
	}
	return ri
}
