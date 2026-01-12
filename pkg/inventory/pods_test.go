package inventory

import (
	"testing"

	"github.com/anchore/k8s-inventory/pkg/client"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFetchPodsInNamespace(t *testing.T) {
	type args struct {
		c         client.Client
		batchSize int64
		timeout   int64
		namespace string
	}
	tests := []struct {
		name    string
		args    args
		want    []v1.Pod
		wantErr bool
	}{
		{
			name: "successfully return pods from namespace",
			args: args{
				c: client.Client{
					Clientset: fake.NewClientset(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation": "test-value",
							},
							Labels: map[string]string{
								"test-label": "test-value",
							},
							Namespace: "test-namespace",
						},
					}),
				},
				batchSize: 100,
				timeout:   10,
				namespace: "test-namespace",
			},
			want: []v1.Pod{
				{ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					UID:  "test-uid",
					Annotations: map[string]string{
						"test-annotation": "test-value",
					},
					Labels: map[string]string{
						"test-label": "test-value",
					},
					Namespace: "test-namespace",
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchPodsInNamespace(tt.args.c, tt.args.batchSize, tt.args.timeout, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProcessPods(t *testing.T) {
	type args struct {
		pods               []v1.Pod
		namespaceUID       string
		nodes              map[string]Node
		includeAnnotations []string
		includeLabels      []string
		disableMetadata    bool
	}
	tests := []struct {
		name string
		args args
		want []Pod
	}{
		{
			name: "successfully return pods",
			args: args{
				pods: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation": "test-value",
							},
							Labels: map[string]string{
								"test-label": "test-value",
							},
							Namespace: "test-namespace",
						},
						Spec: v1.PodSpec{
							NodeName: "test-node",
						},
					},
				},
				namespaceUID: "namespace-uid-0000",
				nodes: map[string]Node{
					"test-node": {
						Name: "test-node",
						UID:  "test-node-uid",
					},
				},
				includeAnnotations: []string{},
				includeLabels:      []string{},
				disableMetadata:    false,
			},
			want: []Pod{
				{
					Name: "test-pod",
					UID:  "test-uid",
					Annotations: map[string]string{
						"test-annotation": "test-value",
					},
					Labels: map[string]string{
						"test-label": "test-value",
					},
					NamespaceUID: "namespace-uid-0000",
					NodeUID:      "test-node-uid",
				},
			},
		},
		{
			name: "successfully return pods without metadata",
			args: args{
				pods: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation": "test-value",
							},
							Labels: map[string]string{
								"test-label": "test-value",
							},
							Namespace: "test-namespace",
						},
						Spec: v1.PodSpec{
							NodeName: "test-node",
						},
					},
				},
				namespaceUID: "namespace-uid-0000",
				nodes: map[string]Node{
					"test-node": {
						Name: "test-node",
						UID:  "test-node-uid",
					},
				},
				includeAnnotations: []string{},
				includeLabels:      []string{},
				disableMetadata:    true,
			},
			want: []Pod{
				{
					Name:         "test-pod",
					UID:          "test-uid",
					NamespaceUID: "namespace-uid-0000",
					NodeUID:      "test-node-uid",
				},
			},
		},
		{
			name: "successfully return pods with filtered annotations/labels",
			args: args{
				pods: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation": "test-value",
								"do-not-include":  "do-not-include",
							},
							Labels: map[string]string{
								"test-label":     "test-value",
								"do-not-include": "do-not-include",
							},
							Namespace: "test-namespace",
						},
						Spec: v1.PodSpec{
							NodeName: "test-node",
						},
					},
				},
				namespaceUID: "namespace-uid-0000",
				nodes: map[string]Node{
					"test-node": {
						Name: "test-node",
						UID:  "test-node-uid",
					},
				},
				includeAnnotations: []string{"test-.*"},
				includeLabels:      []string{"test-.*"},
				disableMetadata:    false,
			},
			want: []Pod{
				{
					Name:         "test-pod",
					UID:          "test-uid",
					NamespaceUID: "namespace-uid-0000",
					NodeUID:      "test-node-uid",
					Annotations: map[string]string{
						"test-annotation": "test-value",
					},
					Labels: map[string]string{
						"test-label": "test-value",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProcessPods(tt.args.pods, tt.args.namespaceUID, tt.args.nodes, tt.args.includeAnnotations, tt.args.includeLabels, tt.args.disableMetadata)
			assert.Equal(t, tt.want, got)
		})
	}
}
