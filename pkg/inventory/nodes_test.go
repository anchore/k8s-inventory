package inventory

import (
	"testing"

	"github.com/anchore/k8s-inventory/pkg/client"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFetchNodes(t *testing.T) {
	type args struct {
		c                  client.Client
		batchSize          int64
		timeout            int64
		includeAnnotations []string
		includeLabels      []string
		disableMetadata    bool
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]Node
		wantErr bool
	}{
		{
			name: "successfully returns nodes",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(&v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation": "test-value",
							},
							Labels: map[string]string{
								"test-label": "test-value",
							},
						},
						Status: v1.NodeStatus{
							NodeInfo: v1.NodeSystemInfo{
								Architecture:            "arm64",
								ContainerRuntimeVersion: "docker://20.10.23",
								KernelVersion:           "5.15.49-linuxkit",
								KubeletVersion:          "v1.26.1",
								OperatingSystem:         "linux",
							},
						},
					}),
				},
				batchSize:          100,
				timeout:            100,
				includeAnnotations: []string{},
				includeLabels:      []string{},
				disableMetadata:    false,
			},
			want: map[string]Node{
				"test-node": {
					Name: "test-node",
					UID:  "test-uid",
					Annotations: map[string]string{
						"test-annotation": "test-value",
					},
					Labels: map[string]string{
						"test-label": "test-value",
					},
					Arch:                    "arm64",
					ContainerRuntimeVersion: "docker://20.10.23",
					KernelVersion:           "5.15.49-linuxkit",
					KubeletVersion:          "v1.26.1",
					OperatingSystem:         "linux",
				},
			},
		},
		{
			name: "successfully returns nodes without metadata",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(&v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation": "test-value",
							},
							Labels: map[string]string{
								"test-label": "test-value",
							},
						},
						Status: v1.NodeStatus{
							NodeInfo: v1.NodeSystemInfo{
								Architecture:            "arm64",
								ContainerRuntimeVersion: "docker://20.10.23",
								KernelVersion:           "5.15.49-linuxkit",
								KubeletVersion:          "v1.26.1",
								OperatingSystem:         "linux",
							},
						},
					}),
				},
				batchSize:          100,
				timeout:            100,
				includeAnnotations: []string{},
				includeLabels:      []string{},
				disableMetadata:    true,
			},
			want: map[string]Node{
				"test-node": {
					Name:                    "test-node",
					UID:                     "test-uid",
					Arch:                    "arm64",
					ContainerRuntimeVersion: "docker://20.10.23",
					KernelVersion:           "5.15.49-linuxkit",
					KubeletVersion:          "v1.26.1",
					OperatingSystem:         "linux",
				},
			},
		},
		{
			name: "successfully returns nodes with filtered annotation/label metadata",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(&v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation":   "test-value",
								"test-annotation-2": "test-value-2",
							},
							Labels: map[string]string{
								"test-label":   "test-value",
								"test-label-2": "test-value-2",
							},
						},
						Status: v1.NodeStatus{
							NodeInfo: v1.NodeSystemInfo{
								Architecture:            "arm64",
								ContainerRuntimeVersion: "docker://20.10.23",
								KernelVersion:           "5.15.49-linuxkit",
								KubeletVersion:          "v1.26.1",
								OperatingSystem:         "linux",
							},
						},
					}),
				},
				batchSize:          100,
				timeout:            100,
				includeAnnotations: []string{".*-2$"},
				includeLabels:      []string{".*-2$"},
				disableMetadata:    false,
			},
			want: map[string]Node{
				"test-node": {
					Name:                    "test-node",
					UID:                     "test-uid",
					Arch:                    "arm64",
					ContainerRuntimeVersion: "docker://20.10.23",
					KernelVersion:           "5.15.49-linuxkit",
					KubeletVersion:          "v1.26.1",
					OperatingSystem:         "linux",
					Annotations: map[string]string{
						"test-annotation-2": "test-value-2",
					},
					Labels: map[string]string{
						"test-label-2": "test-value-2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchNodes(tt.args.c, tt.args.batchSize, tt.args.timeout, tt.args.includeAnnotations, tt.args.includeLabels, tt.args.disableMetadata)
			if (err != nil) != tt.wantErr {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
