package inventory

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultIgnoreNotRunning = true
	defaultMissingTagPolicy = "digest"
	defualtDummyTag         = "UNKNOWN"
)

func logout(actual, expected ReportItem, t *testing.T) {
	t.Log("")
	t.Log("Actual")
	for _, image := range actual.Images {
		t.Logf("  %#v", image)
	}
	t.Log("")
	t.Log("Expected")
	for _, image := range expected.Images {
		t.Logf("  %#v", image)
	}
	t.Log("")
}

func equivalent(left, right ReportItem) error {
	if left.Namespace != right.Namespace {
		return fmt.Errorf("Namespaces do not match %s != %s", left.Namespace, right.Namespace)
	}

	if len(left.Images) != len(right.Images) {
		return fmt.Errorf("Mismatch in number of images %d != %d", len(left.Images), len(right.Images))
	}

	tmap := make(map[string]struct{})
	for _, image := range right.Images {
		key := fmt.Sprintf("%s@%s", image.Tag, image.RepoDigest)
		tmap[key] = struct{}{}
	}

	for _, image := range left.Images {
		key := fmt.Sprintf("%s@%s", image.Tag, image.RepoDigest)
		_, exists := tmap[key]
		if !exists {
			return fmt.Errorf("Actual key %s not found in expected results", key)
		}
	}
	return nil
}

//
//	Test out two containers with the same tag but different digests to ensure
//	the uniqueness of images is parsed correctly when looking at a list of
//	containers in a single pod
//
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, defaultMissingTagPolicy, defualtDummyTag)

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
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out two pods running containers with the same tag but different digests
//	to ensure the uniqueness of images is parsed correctly when looking at
//	individual pods in the same namespace
//
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
				Phase: "Running",
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
				Phase: "Running",
			},
		},
	}
	actual := NewReportItem(mockPods, namespace, defaultIgnoreNotRunning, defaultMissingTagPolicy, defualtDummyTag)

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
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out a pod running with an image added by digest only.
//
//	MissingTagPolicy == "digest"
//
//	kubectl run alpiney --image=alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba
//
func TestAddImageWithDigestNoTagMTPAsDigest(t *testing.T) {
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
				RepoDigest: "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out a pod running with an image added by digest only.
//
//	MissingTagPolicy == "insert"
//	DummyTag == "UNKNOWN"
//
//	kubectl run alpiney --image=alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba
//
func TestAddImageWithDigestNoTagMTPAsInsert(t *testing.T) {
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, "insert", defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        fmt.Sprintf("alpine:%s", defualtDummyTag),
				RepoDigest: "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out a pod running with an image added by digest only.
//
//	MissingTagPolicy == "drop"
//
//	kubectl run alpiney --image=alpine@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba
//
func TestAddImageWithDigestNoTagMTPAsDrop(t *testing.T) {
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, "drop", defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out a pod running with an image added by tag and digest.
//
//	kubectl run alpiney --image=alpine:3.13.6@sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba
//
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3.13.6",
				RepoDigest: "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out a pod running without a tag or digest (inferred 'latest')
//
//	kubectl run alpiney --image=alpine
//
//	TODO: Find where in the runtime this is actually inferred as latest
//
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out a pod running with an image and tag but no digest.
//
//	kubectl run alpiney --image=alpine:3
//
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test when there is no repo info in the ImageID
//
func TestAddImageNoDigestNoRepoInImageID(t *testing.T) {
	namespace := "default"
	mockPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "alpine",
					Image: "alpine:123",
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:    "alpine",
					Image:   "alpine:123",
					ImageID: "docker://sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
				},
			},
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:123",
				RepoDigest: "",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out a pod running with an init container
//
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
			Phase: "Running",
		},
	}
	actual := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	actual.extractUniqueImages(mockPod, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out NewReportItem which takes a list of pods. Include pods with init
//	containers and regular containers.
//
func TestNewReportItem(t *testing.T) {
	namespace := "default"
	mockPods := []v1.Pod{
		{ // pod-1
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
				Phase: "Running",
			},
		},
		{ // pod-2
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
				Phase: "Running",
			},
		},
		{ // pod-3
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
				Phase: "Running",
			},
		},
	}
	actual := NewReportItem(mockPods, namespace, defaultIgnoreNotRunning, defaultMissingTagPolicy, defualtDummyTag)

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
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out NewReportItem which takes a list of pods. Include pods with init
//	containers and regular containers. Include a pod that is in a Pending state
//	that should be ignored.
//
func TestNewReportItemNotRunningTrue(t *testing.T) {
	namespace := "default"
	mockPods := []v1.Pod{
		{ // pod-1
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
				Phase: "Running",
			},
		},
		{ // pod-2
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
				Phase: "Running",
			},
		},
		{ // pod-3
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
				Phase: "Pending",
			},
		},
	}
	actual := NewReportItem(mockPods, namespace, defaultIgnoreNotRunning, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images: []ReportImage{
			{
				Tag:        "alpine:3",
				RepoDigest: "sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a",
			},
		},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out NewReportItem which takes a list of pods. Include pods with init
//	containers and regular containers. Include a pod that is in a Pending state
//	that should still be captured.
//
func TestNewReportItemNotRunningFalse(t *testing.T) {
	namespace := "default"
	mockPods := []v1.Pod{
		{ // pod-1
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
				Phase: "Running",
			},
		},
		{ // pod-2
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
				Phase: "Running",
			},
		},
		{ // pod-3
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
				Phase: "Pending",
			},
		},
	}
	actual := NewReportItem(mockPods, namespace, false, defaultMissingTagPolicy, defualtDummyTag)

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
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out NewReportItem with pods that are for some reason empty
//
func TestNewReportItemEmptyPods(t *testing.T) {
	namespace := "default"
	mockPods := []v1.Pod{
		{ // pod-1
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				InitContainers: []v1.Container{},
			},
			Status: v1.PodStatus{
				InitContainerStatuses: []v1.ContainerStatus{},
				Phase:                 "Running",
			},
		},
		{ // pod-2
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{},
				Phase:             "Running",
			},
		},
		{ // pod-3
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{},
				Phase:             "Pending",
			},
		},
	}
	actual := NewReportItem(mockPods, namespace, defaultIgnoreNotRunning, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}

//
//	Test out NewReportItem with an empty list of pods
//
func TestNewReportItemEmptyPodList(t *testing.T) {
	namespace := "default"
	mockPods := []v1.Pod{}
	actual := NewReportItem(mockPods, namespace, defaultIgnoreNotRunning, defaultMissingTagPolicy, defualtDummyTag)

	expected := ReportItem{
		Namespace: namespace,
		Images:    []ReportImage{},
	}
	err := equivalent(actual, expected)
	if err != nil {
		logout(actual, expected, t)
		t.Error(err)
	}
}
