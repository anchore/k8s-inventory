package result

import v1 "k8s.io/api/core/v1"

// Represents a Namespace Images list result
type Namespace struct {
	Namespace string   `json:"namespace"`
	Images    []string `json:"images"`
}

func NewNamespace(pod v1.Pod) *Namespace {
	return &Namespace{
		Namespace: pod.Namespace,
		Images:    getUniqueImagesFromPodSpec(pod.Spec.Containers),
	}
}

func (n *Namespace) AddImages(podSpec v1.PodSpec) {
	if len(n.Images) == 0 {
		n.Images = getUniqueImagesFromPodSpec(podSpec.Containers)
	} else {
		// Build a Map to make use as a Set (unique list). Values are empty structs so they don't waste space
		imageSet := make(map[string]struct{})
		for _, image := range n.Images {
			imageSet[image] = struct{}{}
		}
		// If the image isn't in the set already, append it to the list
		for _, image := range getUniqueImagesFromPodSpec(podSpec.Containers) {
			if _, ok := imageSet[image]; !ok {
				n.Images = append(n.Images, image)
			}
		}
	}
}

func getUniqueImagesFromPodSpec(containers []v1.Container) []string {
	imageMap := make(map[string]struct{})
	for _, container := range containers {
		imageMap[container.Image] = struct{}{}
	}
	imageSlice := make([]string, 0, len(imageMap))
	for k := range imageMap {
		imageSlice = append(imageSlice, k)
	}
	return imageSlice
}
