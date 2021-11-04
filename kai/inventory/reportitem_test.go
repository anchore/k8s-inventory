package inventory

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func equivalent(left, right ReportItem, t *testing.T) error {
	if left.Namespace != right.Namespace {
		return fmt.Errorf("Namespaces do not match %s != %s", left.Namespace, right.Namespace)
	}

	if len(left.Images) != len(right.Images) {
		return fmt.Errorf("Mismatch in number of images %d != %d", len(left.Images), len(right.Images))
	}

	tmap := make(map[string]struct{})
	for _, image := range left.Images {
		key := fmt.Sprintf("%s@%s", image.Tag, image.RepoDigest)
		tmap[key] = struct{}{}
	}

	for _, image := range right.Images {
		key := fmt.Sprintf("%s@%s", image.Tag, image.RepoDigest)
		_, exists := tmap[key]
		if !exists {
			return fmt.Errorf("Mismatch in ReportItem Images array %s does not exist", key)
		}
	}
	return nil
}

func TestSameTagDifferentDigestSamePod(t *testing.T) {
	namespace := "default"
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "sametag-alpine",
					Image: "jpetersenames/sametag:latest",
				},
				{
					Name:  "sametag-centos",
					Image: "jpetersenames/sametag:latest",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:    "sametag-alpine",
					Image:   "jpetersenames/sametag:latest",
					ImageID: "docker-pullable://jpetersenames/sametag@sha256:5762a7f909e42866c63570f3107e2ab9d6d39309233f4312bb40c3b68aaf4f8a",
				},
				{
					Name:    "sametag-centos",
					Image:   "jpetersenames/sametag:latest",
					ImageID: "docker-pullable://jpetersenames/sametag@sha256:a0b39cd754f1236114a1603ee1791deb660c78bb963da1f6aed48807c796b9d1",
				},
			},
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod)

	for _, image := range actual.Images {
		t.Log(image)
	}

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "jpetersenames/sametag:latest",
				RepoDigest: "sha256:5762a7f909e42866c63570f3107e2ab9d6d39309233f4312bb40c3b68aaf4f8a",
			},
			{
				Tag:        "jpetersenames/sametag:latest",
				RepoDigest: "sha256:a0b39cd754f1236114a1603ee1791deb660c78bb963da1f6aed48807c796b9d1",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}

func TestSameTagDifferentDigestDistinctPods(t *testing.T) {
	namespace := "default"
	mockPods := []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "sametag-centos",
						Image: "jpetersenames/sametag:latest",
					},
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name:    "sametag-centos",
						Image:   "jpetersenames/sametag:latest",
						ImageID: "docker-pullable://jpetersenames/sametag@sha256:a0b39cd754f1236114a1603ee1791deb660c78bb963da1f6aed48807c796b9d1",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "sametag-alpine",
						Image: "jpetersenames/sametag:latest",
					},
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name:    "sametag-alpine",
						Image:   "jpetersenames/sametag:latest",
						ImageID: "docker-pullable://jpetersenames/sametag@sha256:5762a7f909e42866c63570f3107e2ab9d6d39309233f4312bb40c3b68aaf4f8a",
					},
				},
			},
		},
	}
	actual := NewReportItem(mockPods, namespace)

	for _, image := range actual.Images {
		t.Log(image)
	}

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "jpetersenames/sametag:latest",
				RepoDigest: "sha256:5762a7f909e42866c63570f3107e2ab9d6d39309233f4312bb40c3b68aaf4f8a",
			},
			{
				Tag:        "jpetersenames/sametag:latest",
				RepoDigest: "sha256:a0b39cd754f1236114a1603ee1791deb660c78bb963da1f6aed48807c796b9d1",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}

// kubectl run alpiney --image=alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba
func TestAddImageWithDigestNoTag(t *testing.T) {
	namespace := "default"
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "alpine1",
					Image: "alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:    "alpine1",
					Image:   "alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
					ImageID: "docker-pullable://alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
				},
			},
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod)

	// TODO: What should the null tag be?
	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:", // TODO: This needs to change when the null tag is decided
				RepoDigest: "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}

// kubectl run alpiney --image=alpine:3.13.6@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba
func TestAddImageWithDigestWithTag(t *testing.T) {
	namespace := "default"
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "alpine2",
					Image: "alpine:3.13.6@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name: "alpine2",
					// For some reason k8s makes this the image id...
					Image:   "sha256:2d1d6881767e3e1c194b061b3422aa76bf076aefd51d1d27c679ff998ead3104",
					ImageID: "docker-pullable://alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
				},
			},
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3.13.6",
				RepoDigest: "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}

// kubectl run alpiney --image=alpine
func TestAddImageNoDigestNoTag(t *testing.T) {
	namespace := "default"
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "alpine3",
					Image: "alpine",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:    "alpine3",
					Image:   "alpine:3", // TODO: Check this when rate limiting subsides
					ImageID: "docker-pullable://alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
				},
			},
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}

// kubectl run alpiney --image=alpine:3
func TestAddImageNoDigestWithTag(t *testing.T) {
	namespace := "default"
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "alpine4",
					Image: "alpine:3",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:    "alpine4",
					Image:   "alpine:3",
					ImageID: "docker-pullable://alpine@sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
				},
			},
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}

// kubectl run alpiney --image=alpine:3
func TestInitContainer(t *testing.T) {
	namespace := "default"
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{
					Name:  "alpine-init",
					Image: "alpine:3",
				},
			},
		},
		Status: v1.PodStatus{
			InitContainerStatuses: []v1.ContainerStatus{
				{
					Name:    "alpine-init",
					Image:   "alpine:3",
					ImageID: "docker-pullable://alpine@sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
				},
			},
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}

// kubectl run alpiney --image=alpine:3
func TestNewReportItem(t *testing.T) {
	namespace := "default"
	mockPods := []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				InitContainers: []v1.Container{
					{
						Name:  "alpine-init",
						Image: "alpine:3",
					},
				},
			},
			Status: v1.PodStatus{
				InitContainerStatuses: []v1.ContainerStatus{
					{
						Name:    "alpine-init",
						Image:   "alpine:3",
						ImageID: "docker-pullable://alpine@sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "alpine4",
						Image: "alpine:3",
					},
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name:    "alpine4",
						Image:   "alpine:3",
						ImageID: "docker-pullable://alpine@sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "sametag-alpine",
						Image: "jpetersenames/sametag:latest",
					},
					{
						Name:  "sametag-centos",
						Image: "jpetersenames/sametag:latest",
					},
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						Name:    "sametag-alpine",
						Image:   "jpetersenames/sametag:latest",
						ImageID: "docker-pullable://jpetersenames/sametag@sha256:5762a7f909e42866c63570f3107e2ab9d6d39309233f4312bb40c3b68aaf4f8a",
					},
					{
						Name:    "sametag-centos",
						Image:   "jpetersenames/sametag:latest",
						ImageID: "docker-pullable://jpetersenames/sametag@sha256:a0b39cd754f1236114a1603ee1791deb660c78bb963da1f6aed48807c796b9d1",
					},
				},
			},
		},
	}
	actual := NewReportItem(mockPods, namespace)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
			},
			{
				Tag:        "jpetersenames/sametag:latest",
				RepoDigest: "sha256:5762a7f909e42866c63570f3107e2ab9d6d39309233f4312bb40c3b68aaf4f8a",
			},
			{
				Tag:        "jpetersenames/sametag:latest",
				RepoDigest: "sha256:a0b39cd754f1236114a1603ee1791deb660c78bb963da1f6aed48807c796b9d1",
			},
		},
	}
	err := equivalent(actual, expected, t)
	if err != nil {
		t.Error(err)
	}
}
