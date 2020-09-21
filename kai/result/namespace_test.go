package result

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
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

	actualNamespace := NewNamespace(mockPod)

	expectedNamespace := Namespace{
		Namespace: "default",
		Images: []Image{
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
				RepoDigest: "",
			},
		},
	}

	if actualNamespace.Namespace != expectedNamespace.Namespace {
		t.Errorf("Namespaces do not match:\nexpected=%s\nactual=%s", expectedNamespace.Namespace, actualNamespace.Namespace)
	}

	if !reflect.DeepEqual(actualNamespace.Images, expectedNamespace.Images) {
		t.Errorf("Image Lists do not match:\nexpected=%v\nactual=%v", expectedNamespace.Images, actualNamespace.Images)
	}
}

func TestAddImages(t *testing.T) {
	expectedImages := []Image{
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
			RepoDigest: "",
		},
	}

	namespace := Namespace{
		Namespace: "default",
		Images:    []Image{},
	}
	namespace.AddImages(v1.Pod{
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

	if !reflect.DeepEqual(expectedImages, namespace.Images) {
		t.Errorf("Image Lists do not match:\nexpected=%v\nactual=%v", expectedImages, namespace.Images)
	}
}
