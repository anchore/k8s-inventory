package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func Test_getContainersInPod(t *testing.T) {
	type args struct {
		pod v1.Pod
	}
	tests := []struct {
		name string
		args args
		want []Container
	}{
		{
			name: "successfully returns regular containers",
			args: args{
				pod: v1.Pod{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "test-container",
								Image: "docker.io/kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
							},
						},
					},
					Status: v1.PodStatus{
						ContainerStatuses: []v1.ContainerStatus{
							{
								Name:        "test-container",
								Image:       "sha256:20b332c9a70d8516d849d1ac23eff5800cbb2f263d379f0ec11ee908db6b25a8",
								ImageID:     "docker-pullable://kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
								ContainerID: "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
							},
						},
					},
				},
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "docker.io/kubernetesui/dashboard:v2.7.0",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
		{
			name: "successfully returns init containers",
			args: args{
				pod: v1.Pod{
					Spec: v1.PodSpec{
						InitContainers: []v1.Container{
							{
								Name:  "test-container",
								Image: "docker.io/kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
							},
						},
					},
					Status: v1.PodStatus{
						InitContainerStatuses: []v1.ContainerStatus{
							{
								Name:        "test-container",
								Image:       "sha256:20b332c9a70d8516d849d1ac23eff5800cbb2f263d379f0ec11ee908db6b25a8",
								ImageID:     "docker-pullable://kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
								ContainerID: "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
							},
						},
					},
				},
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "docker.io/kubernetesui/dashboard:v2.7.0",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
		{
			name: "successfully returns with an image tag if spec is missing",
			args: args{
				pod: v1.Pod{
					Status: v1.PodStatus{
						InitContainerStatuses: []v1.ContainerStatus{
							{
								Name:        "test-container",
								Image:       "anchore/test:v1.0.0",
								ImageID:     "docker-pullable://anchore/test@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
								ContainerID: "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
							},
						},
					},
				},
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "anchore/test:v1.0.0",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContainersInPod(tt.args.pod)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetContainersFromPods(t *testing.T) {
	type args struct {
		pods             []v1.Pod
		ignoreNotRunning bool
	}
	tests := []struct {
		name string
		args args
		want []Container
	}{
		{
			name: "successfully returns all containers",
			args: args{
				pods: []v1.Pod{
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "docker.io/kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
								},
							},
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "test-container",
									Image:       "sha256:20b332c9a70d8516d849d1ac23eff5800cbb2f263d379f0ec11ee908db6b25a8",
									ImageID:     "docker-pullable://kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
									ContainerID: "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
								},
							},
							Phase: v1.PodRunning,
						},
					},
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container2",
									Image: "anchore/kai:v1.0.0",
								},
							},
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "test-container2",
									Image:       "anchore/kai:v1.0.0",
									ImageID:     "docker-pullable://anchore/kai@sha256:9999999wwwwwwwwwwwwwwwwffffffffffffff",
									ContainerID: "docker://a9cd75ad000000000000000000003b58c62bfd7b03cabeb764c1dac97568ad26",
								},
							},
							Phase: v1.PodPending,
						},
					},
				},
				ignoreNotRunning: false,
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "docker.io/kubernetesui/dashboard:v2.7.0",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
				{
					Name:        "test-container2",
					ImageTag:    "anchore/kai:v1.0.0",
					ImageDigest: "sha256:9999999wwwwwwwwwwwwwwwwffffffffffffff",
					ID:          "docker://a9cd75ad000000000000000000003b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
		{
			name: "only running containers successfully returned",
			args: args{
				pods: []v1.Pod{
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "docker.io/kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
								},
							},
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "test-container",
									Image:       "sha256:20b332c9a70d8516d849d1ac23eff5800cbb2f263d379f0ec11ee908db6b25a8",
									ImageID:     "docker-pullable://kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
									ContainerID: "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
								},
							},
							Phase: v1.PodRunning,
						},
					},
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container2",
									Image: "anchore/kai:v1.0.0",
								},
							},
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "test-container2",
									Image:       "anchore/kai:v1.0.0",
									ImageID:     "docker-pullable://anchore/kai@sha256:9999999wwwwwwwwwwwwwwwwffffffffffffff",
									ContainerID: "docker://a9cd75ad000000000000000000003b58c62bfd7b03cabeb764c1dac97568ad26",
								},
							},
							Phase: v1.PodPending,
						},
					},
				},
				ignoreNotRunning: true,
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "docker.io/kubernetesui/dashboard:v2.7.0",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetContainersFromPods(tt.args.pods, tt.args.ignoreNotRunning)
			assert.Equal(t, tt.want, got)
		})
	}
}
