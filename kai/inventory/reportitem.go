package inventory

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anchore/kai/internal/log"

	v1 "k8s.io/api/core/v1"
)

// ReportItem represents a ReportItem Images list result
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
func NewReportItem(pods []v1.Pod, namespace string) ReportItem {

	reportItem := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}

	for _, pod := range pods {
		namespace := pod.ObjectMeta.Namespace
		if namespace == "" || len(pod.Status.ContainerStatuses) == 0 {
			continue
		}
		reportItem.extractUniqueImages(pod)
	}

	return reportItem
}

// Represent the namespace as a string
func (r *ReportItem) String() string {
	return fmt.Sprintf("ReportItem(namespace=%s, images=%v)", r.Namespace, r.Images)
}

// Adds an ReportImage to the ReportItem struct (if it doesn't exist there already)
func (r *ReportItem) extractUniqueImages(pod v1.Pod) {

	if len(r.Images) == 0 {
		r.Images = process(pod)

	} else {

		// Build a Map to make use as a Set (unique list). Values are empty structs so they don't waste space
		imageSet := make(map[string]ReportImage)
		for _, image := range r.Images {
			// TODO: Use the image:tag@digest to test for uniqueness
			imageSet[image.Tag] = image
		}

		// If the image isn't in the set already, append it to the list
		for _, image := range process(pod) {
			if _, ok := imageSet[image.Tag]; !ok {
				r.Images = append(r.Images, image)
			}
		}
	}
}

const regexTag = `:[\w][\w.-]{0,127}$`
const regexDigest = `[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}`

type image struct {
	digest string
	tag    string
	image  string
	key    string
}

func fill(pod v1.Pod) map[string][]string {
	cmap := make(map[string][]string)

	// grab init images
	for _, c := range pod.Spec.InitContainers {
		cmap[c.Name] = append(cmap[c.Name], c.Image)
	}

	for _, c := range pod.Status.InitContainerStatuses {
		cmap[c.Name] = append(cmap[c.Name], c.Image, c.ImageID)
	}

	// grab regular images
	for _, c := range pod.Spec.Containers {
		cmap[c.Name] = append(cmap[c.Name], c.Image)
	}

	for _, c := range pod.Status.ContainerStatuses {
		cmap[c.Name] = append(cmap[c.Name], c.Image, c.ImageID)
	}

	return cmap
}

func parseout(s string, c *image) error {

	if c.digest != "" && c.tag != "" && c.image != "" {
		log.Info("all fields have been parsed")
		return nil
	}

	reg, err := regexp.Compile(regexTag)
	if err != nil {
		return err
	}

	image := s
	digest := ""
	if strings.Contains(s, "@") {
		split := strings.Split(s, "@")
		image = split[0]
		digest = split[1]
	}

	tag := ""
	m := reg.FindAllString(image, -1)
	if len(m) > 0 {
		i := strings.LastIndex(image, ":")
		image = image[0:i]
		tag = strings.TrimPrefix(m[0], ":")
	}

	if c.digest == "" {
		c.digest = digest
	}
	if c.tag == "" {
		c.tag = tag
	}
	if c.image == "" {
		c.image = image
	}
	return nil
}

func extract(imagedata []string) (image, error) {

	c := image{
		image:  "",
		tag:    "",
		digest: "",
		key:    "",
	}

	for _, data := range imagedata {
		parseout(data, &c)
	}

	c.key = fmt.Sprintf("%s:%s@%s", c.image, c.tag, c.digest)
	return c, nil
}

func process(pod v1.Pod) []ReportImage {
	cmap := fill(pod)

	unique := make(map[string]image)
	for _, n := range cmap {
		c, err := extract(n)
		if err != nil {
			log.Errorf("issue processing %s", n)
			continue
		}
		unique[c.key] = c
	}

	ri := make([]ReportImage, 0)
	for _, u := range unique {
		ri = append(ri, ReportImage{
			Tag:        fmt.Sprintf("%s:%s", u.image, u.tag),
			RepoDigest: u.digest,
		})
	}
	return ri
}
