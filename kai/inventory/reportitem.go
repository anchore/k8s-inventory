package inventory

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// Represents a ReportItem Images list result
type ReportItem struct {
	Namespace string        `json:"namespace,omitempty"`
	Images    []ReportImage `json:"images"`
}

type ReportImage struct {
	Tag        string `json:"tag,omitempty"`
	RepoDigest string `json:"repoDigest,omitempty"`
}

func NewFromPod(pod v1.Pod) *ReportItem {
	return &ReportItem{
		Namespace: pod.Namespace,
		Images:    getUniqueImagesFromPodStatus(pod),
	}
}

func New(namespace string) *ReportItem {
	return &ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
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
		reportItem.AddImages(pod)
	}

	return reportItem
}

// Represent the namespace as a string
func (n *ReportItem) String() string {
	return fmt.Sprintf("ReportItem(namespace=%s, images=%v)", n.Namespace, n.Images)
}

// Adds an ReportImage to the ReportItem struct (if it doesn't exist there already)
func (n *ReportItem) AddImages(pod v1.Pod) {
	if len(n.Images) == 0 {
		n.Images = getUniqueImagesFromPodStatus(pod)
	} else {
		// Build a Map to make use as a Set (unique list). Values are empty structs so they don't waste space
		imageSet := make(map[string]ReportImage)
		for _, image := range n.Images {
			// There's always a tag, the repoDigest may be missing
			imageSet[image.Tag] = image
		}
		// If the image isn't in the set already, append it to the list
		for _, image := range getUniqueImagesFromPodStatus(pod) {
			if _, ok := imageSet[image.Tag]; !ok {
				n.Images = append(n.Images, image)
			}
		}
	}
}

func getUniqueImagesFromPodStatus(pod v1.Pod) []ReportImage {
	imageMap := make(map[string]ReportImage)
	for _, container := range pod.Status.ContainerStatuses {
		repoDigest := getImageDigest(container.ImageID)
		imageMap[container.Image] = ReportImage{
			Tag:        container.Image,
			RepoDigest: repoDigest,
		}
	}
	imageSlice := make([]ReportImage, 0, len(imageMap))
	for _, v := range imageMap {
		imageSlice = append(imageSlice, v)
	}
	return imageSlice
}

func getImageDigest(imageID string) string {
	var imageDigest = ""
	// If the image ID contains "sha", it corresponds to the repo digest. If not, it's not a digest
	if strings.Contains(imageID, "sha") {
		imageDigest = "sha" + strings.Split(imageID, "sha")[1]
	}
	return imageDigest
}
