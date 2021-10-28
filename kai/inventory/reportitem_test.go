package inventory

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/magiconair/properties/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConstructorFromPod(t *testing.T) {
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Image:   "dakaneye/test:1.0.0",
					ImageID: "docker-pullable://dakaneye/test@sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd", // note this isn't the real digest
				},
				{
					Image:   "k8s.gcr.io/coredns:1.6.2",
					ImageID: "docker-pullable://k8s.gcr.io/coredns@sha256:12eb885b8685b1b13a04ecf5c23bc809c2e57917252fd7b0be9e9c00644e8ee5", // real
				},
				{
					Image:   "k8s.gcr.io/coredns:1.6.2",
					ImageID: "docker-pullable://k8s.gcr.io/coredns@sha256:12eb885b8685b1b13a04ecf5c23bc809c2e57917252fd7b0be9e9c00644e8ee5",
				},
				{
					Image:   "localhost/samtest:latest",
					ImageID: "docker://sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd",
				},
			},
		},
	}

	actual := NewFromPod(mockPod)

	expected := ReportItem{
		Namespace: "default",
		Images: []ReportImage{
			{
				Tag:        "dakaneye/test:1.0.0",
				RepoDigest: "sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd", // not real
			},
			{
				Tag:        "k8s.gcr.io/coredns:1.6.2",
				RepoDigest: "sha256:12eb885b8685b1b13a04ecf5c23bc809c2e57917252fd7b0be9e9c00644e8ee5", // real
			},
			{
				Tag:        "localhost/samtest:latest",
				RepoDigest: "sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd",
			},
		},
	}

	if actual.Namespace != expected.Namespace {
		t.Errorf("Namespaces do not match:\nexpected=%s\nactual=%s", expected.Namespace, actual.Namespace)
	}

	compareImageSlices(expected.Images, actual.Images, t)
}

func TestAddImages(t *testing.T) {
	expected := []ReportImage{
		{
			Tag:        "dakaneye/test:1.0.0",
			RepoDigest: "sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd", // not real
		},
		{
			Tag:        "k8s.gcr.io/coredns:1.6.2",
			RepoDigest: "sha256:12eb885b8685b1b13a04ecf5c23bc809c2e57917252fd7b0be9e9c00644e8ee5", // real
		},
		{
			Tag:        "localhost/samtest:latest",
			RepoDigest: "sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd",
		},
	}

	actual := ReportItem{
		Namespace: "default",
		Images:    []ReportImage{},
	}
	actual.AddImages(v1.Pod{
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Image:   "dakaneye/test:1.0.0",
					ImageID: "docker-pullable://dakaneye/test@sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd", // note this isn't the real digest
				},
				{
					Image:   "k8s.gcr.io/coredns:1.6.2",
					ImageID: "docker-pullable://k8s.gcr.io/coredns@sha256:12eb885b8685b1b13a04ecf5c23bc809c2e57917252fd7b0be9e9c00644e8ee5", // real
				},
				{
					Image:   "k8s.gcr.io/coredns:1.6.2",
					ImageID: "docker-pullable://k8s.gcr.io/coredns@sha256:12eb885b8685b1b13a04ecf5c23bc809c2e57917252fd7b0be9e9c00644e8ee5",
				},
				{
					Image:   "localhost/samtest:latest",
					ImageID: "docker://sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd",
				},
			},
		},
	})
	compareImageSlices(expected, actual.Images, t)
}

func compareImageSlices(expected []ReportImage, actual []ReportImage, t *testing.T) {
	// Couldn't find something that did good equality comparisons on slices (regardless of order)
	// So, load images expected into a map, and compare them one by one against actual images added
	expectedImagesMap := make(map[string]ReportImage)
	for _, expectedImage := range expected {
		expectedImagesMap[expectedImage.Tag] = expectedImage
	}

	matches := 0
	for _, actualImage := range actual {
		expected, ok := expectedImagesMap[actualImage.Tag]
		if !ok {
			t.Errorf("Unexpected Image Tag added: %v", actualImage)
			return
		}
		// Tags must have already matched
		if expected.RepoDigest == actualImage.RepoDigest {
			matches++
		} else {
			t.Errorf("Image Digests don't match:\nexpected=%s\nactual=%s", expected.RepoDigest, actualImage.RepoDigest)
			return
		}
	}
	if matches != len(expected) {
		diff := deep.Equal(expected, actual)
		t.Error(diff)
	}
}

func TestGetImageDigest(t *testing.T) {
	cases := []struct {
		name     string
		imageID  string
		expected string
	}{
		{
			name:     "common sha256",
			imageID:  "docker.io/anchore/test_images@sha256:f3026e3f808e38c86ffb64e4fc5b49516d0783df2d94f06f959cf8f23c197495",
			expected: "sha256:f3026e3f808e38c86ffb64e4fc5b49516d0783df2d94f06f959cf8f23c197495",
		},
		{
			name:     "common sha512",
			imageID:  "docker.io/anchore/test_images@sha512:72e59bea07d815ee05114b487d9d60594c9b3fc20fa055bff9c09a46ec8c9ff2",
			expected: "sha512:72e59bea07d815ee05114b487d9d60594c9b3fc20fa055bff9c09a46ec8c9ff2",
		},
		{
			name:     "docker-pullable",
			imageID:  "docker-pullable://dakaneye/test@sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd",
			expected: "sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd",
		},
		{
			name:     "docker",
			imageID:  "docker://sha256:ea65104b4b40b5d23eb4b2ebd4f62adf24f714a2fdaff19060de207d1f3c2111",
			expected: "sha256:ea65104b4b40b5d23eb4b2ebd4f62adf24f714a2fdaff19060de207d1f3c2111",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := getImageDigest(test.imageID)
			assert.Equal(t, test.expected, actual)
		})
	}
}
