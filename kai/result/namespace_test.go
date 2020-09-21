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
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Image: "dakaneye/test:1.0.0",
				},
				{
					Image: "k8s.gcr.io/coredns:1.6.2",
				},
				{
					Image: "k8s.gcr.io/coredns:1.6.2",
				},
			},
		},
	}

	actualNamespace := NewNamespace(mockPod)

	expectedNamespace := Namespace{
		Namespace: "default",
		Images: []string{
			"dakaneye/test:1.0.0",
			"k8s.gcr.io/coredns:1.6.2",
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
	expectedImages := []string{
		"dakaneye/test:1.0.0",
		"k8s.gcr.io/coredns:1.6.2",
	}

	namespace := Namespace{
		Namespace: "default",
		Images:    []string{},
	}
	namespace.AddImages(v1.PodSpec{
		Containers: []v1.Container{
			{
				Image: "dakaneye/test:1.0.0",
			},
			{
				Image: "k8s.gcr.io/coredns:1.6.2",
			},
			{
				Image: "k8s.gcr.io/coredns:1.6.2",
			},
		},
	})
	if !reflect.DeepEqual(expectedImages, namespace.Images) {
		t.Errorf("Image Lists do not match:\nexpected=%v\nactual=%v", expectedImages, namespace.Images)
	}
}
