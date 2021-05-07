package result

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// Represents a Namespace Images list result
type Namespace struct {
	Namespace string  `json:"namespace,omitempty"`
	Images    []Image `json:"images"`
}

type Image struct {
	Tag        string `json:"tag,omitempty"`
	RepoDigest string `json:"repoDigest,omitempty"`
}

func NewFromPod(pod v1.Pod) *Namespace {
	return &Namespace{
		Namespace: pod.Namespace,
		Images:    getUniqueImagesFromPodStatus(pod),
	}
}

func New(namespace string) *Namespace {
	return &Namespace{
		Namespace: namespace,
		Images:    []Image{},
	}
}

// Represent the namespace as a string
func (n *Namespace) String() string {
	return fmt.Sprintf("Namespace(namespace=%s, images=%v)", n.Namespace, n.Images)
}

// Adds an Image to the Namespace struct (if it doesn't exist there already)
func (n *Namespace) AddImages(pod v1.Pod) {
	if len(n.Images) == 0 {
		n.Images = getUniqueImagesFromPodStatus(pod)
	} else {
		// Build a Map to make use as a Set (unique list). Values are empty structs so they don't waste space
		imageSet := make(map[string]Image)
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

func getUniqueImagesFromPodStatus(pod v1.Pod) []Image {
	imageMap := make(map[string]Image)
	for _, container := range pod.Status.ContainerStatuses {
		repoDigest := getImageDigest(container.ImageID)
		imageMap[container.Image] = Image{
			Tag:        container.Image,
			RepoDigest: repoDigest,
		}
	}
	imageSlice := make([]Image, 0, len(imageMap))
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
