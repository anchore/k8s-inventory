package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceFilter_Keep(t *testing.T) {
	type args struct {
		excludes []string
		includes []string
	}
	tests := []struct {
		name      string
		args      args
		namespace string
		want      bool
	}{
		{
			name:      "no selectors keeps everything",
			args:      args{},
			namespace: "default",
			want:      true,
		},
		{
			name:      "explicit exclude drops the namespace",
			args:      args{excludes: []string{"default"}},
			namespace: "default",
			want:      false,
		},
		{
			name:      "explicit exclude leaves other namespaces alone",
			args:      args{excludes: []string{"default"}},
			namespace: "kube-system",
			want:      true,
		},
		{
			name:      "regex exclude drops matching namespaces",
			args:      args{excludes: []string{"^kube-*"}},
			namespace: "kube-system",
			want:      false,
		},
		{
			name:      "include acts as an allow-list",
			args:      args{includes: []string{"default"}},
			namespace: "default",
			want:      true,
		},
		{
			name:      "namespaces outside the allow-list are dropped",
			args:      args{includes: []string{"default"}},
			namespace: "kube-system",
			want:      false,
		},
		{
			name:      "exclude takes precedence over include",
			args:      args{excludes: []string{"default"}, includes: []string{"default"}},
			namespace: "default",
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewNamespaceFilter(tt.args.excludes, tt.args.includes)
			assert.Equal(t, tt.want, filter.Keep(tt.namespace))
		})
	}
}

func TestNamespaceFromV1(t *testing.T) {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			UID:  "namespace-uid",
			Annotations: map[string]string{
				"anchore.io/annotation": "annotation-value",
				"other":                 "other-value",
			},
			Labels: map[string]string{
				"anchore.io/label": "label-value",
				"other":            "other-value",
			},
		},
	}

	tests := []struct {
		name               string
		includeAnnotations []string
		includeLabels      []string
		disableMetadata    bool
		want               Namespace
	}{
		{
			name: "all metadata is collected by default",
			want: Namespace{
				Name: "test-namespace",
				UID:  "namespace-uid",
				Annotations: map[string]string{
					"anchore.io/annotation": "annotation-value",
					"other":                 "other-value",
				},
				Labels: map[string]string{
					"anchore.io/label": "label-value",
					"other":            "other-value",
				},
			},
		},
		{
			name:               "metadata is filtered by the include lists",
			includeAnnotations: []string{"anchore.io/.*"},
			includeLabels:      []string{"anchore.io/.*"},
			want: Namespace{
				Name:        "test-namespace",
				UID:         "namespace-uid",
				Annotations: map[string]string{"anchore.io/annotation": "annotation-value"},
				Labels:      map[string]string{"anchore.io/label": "label-value"},
			},
		},
		{
			name:            "metadata collection can be disabled",
			disableMetadata: true,
			want: Namespace{
				Name: "test-namespace",
				UID:  "namespace-uid",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NamespaceFromV1(namespace, tt.includeAnnotations, tt.includeLabels, tt.disableMetadata)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNodeFromV1(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			UID:  "node-uid",
			Annotations: map[string]string{
				"anchore.io/annotation": "annotation-value",
				"other":                 "other-value",
			},
			Labels: map[string]string{
				"anchore.io/label": "label-value",
				"other":            "other-value",
			},
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				Architecture:            "amd64",
				ContainerRuntimeVersion: "containerd://1.7.0",
				KernelVersion:           "5.15.0",
				KubeletVersion:          "v1.30.0",
				OperatingSystem:         "linux",
			},
		},
	}

	tests := []struct {
		name               string
		includeAnnotations []string
		includeLabels      []string
		disableMetadata    bool
		want               Node
	}{
		{
			name: "all metadata is collected by default",
			want: Node{
				Name:                    "test-node",
				UID:                     "node-uid",
				Arch:                    "amd64",
				ContainerRuntimeVersion: "containerd://1.7.0",
				KernelVersion:           "5.15.0",
				KubeletVersion:          "v1.30.0",
				OperatingSystem:         "linux",
				Annotations: map[string]string{
					"anchore.io/annotation": "annotation-value",
					"other":                 "other-value",
				},
				Labels: map[string]string{
					"anchore.io/label": "label-value",
					"other":            "other-value",
				},
			},
		},
		{
			name:               "metadata is filtered by the include lists",
			includeAnnotations: []string{"anchore.io/.*"},
			includeLabels:      []string{"anchore.io/.*"},
			want: Node{
				Name:                    "test-node",
				UID:                     "node-uid",
				Arch:                    "amd64",
				ContainerRuntimeVersion: "containerd://1.7.0",
				KernelVersion:           "5.15.0",
				KubeletVersion:          "v1.30.0",
				OperatingSystem:         "linux",
				Annotations:             map[string]string{"anchore.io/annotation": "annotation-value"},
				Labels:                  map[string]string{"anchore.io/label": "label-value"},
			},
		},
		{
			name:            "metadata collection can be disabled",
			disableMetadata: true,
			want: Node{
				Name:                    "test-node",
				UID:                     "node-uid",
				Arch:                    "amd64",
				ContainerRuntimeVersion: "containerd://1.7.0",
				KernelVersion:           "5.15.0",
				KubeletVersion:          "v1.30.0",
				OperatingSystem:         "linux",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NodeFromV1(node, tt.includeAnnotations, tt.includeLabels, tt.disableMetadata)
			assert.Equal(t, tt.want, got)
		})
	}
}
