package result

// Represents a Namespace Images list result
type Namespace struct {
	Name   string
	Images []string
}

func (n *Namespace) AddImages(images []string) {
	if len(n.Images) == 0 {
		n.Images = images
	} else {
		// Build a Map to make use as a Set (unique list). Values are empty structs so they don't waste space
		imageSet := make(map[string]struct{})
		for _, image := range n.Images {
			imageSet[image] = struct{}{}
		}
		// If the image isn't in the set already, append it to the list
		for _, image := range images {
			if _, ok := imageSet[image]; !ok {
				n.Images = append(n.Images, image)
			}
		}
	}
}
