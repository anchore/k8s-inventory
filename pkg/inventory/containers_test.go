package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func Test_getContainersInPod(t *testing.T) {
	type args struct {
		pod                     v1.Pod
		missingRegistryOverride string
		missingTagPolicy        string
		dummyTag                string
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
				missingRegistryOverride: "",
				missingTagPolicy:        "digest",
				dummyTag:                "UNKNOWN",
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
				missingRegistryOverride: "",
				missingTagPolicy:        "digest",
				dummyTag:                "UNKNOWN",
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
				missingRegistryOverride: "",
				missingTagPolicy:        "digest",
				dummyTag:                "UNKNOWN",
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
			got := getContainersInPod(tt.args.pod, tt.args.missingRegistryOverride, tt.args.missingTagPolicy, tt.args.dummyTag)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetContainersFromPods(t *testing.T) {
	type args struct {
		pods                    []v1.Pod
		ignoreNotRunning        bool
		missingRegistryOverride string
		missingTagPolicy        string
		dummyTag                string
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
				ignoreNotRunning:        false,
				missingRegistryOverride: "",
				missingTagPolicy:        "digest",
				dummyTag:                "",
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
				ignoreNotRunning:        true,
				missingRegistryOverride: "",
				missingTagPolicy:        "digest",
				dummyTag:                "",
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
			name: "successfully returns containers missing tags (digest policy)",
			args: args{
				pods: []v1.Pod{
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "docker.io/kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
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
				},
				ignoreNotRunning:        false,
				missingRegistryOverride: "",
				missingTagPolicy:        "digest",
				dummyTag:                "",
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "docker.io/kubernetesui/dashboard:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
		{
			name: "successfully returns containers missing tags (drop policy)",
			args: args{
				pods: []v1.Pod{
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "docker.io/kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
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
				ignoreNotRunning:        false,
				missingRegistryOverride: "",
				missingTagPolicy:        "drop",
				dummyTag:                "",
			},
			want: []Container{
				{
					Name:        "test-container2",
					ImageTag:    "anchore/kai:v1.0.0",
					ImageDigest: "sha256:9999999wwwwwwwwwwwwwwwwffffffffffffff",
					ID:          "docker://a9cd75ad000000000000000000003b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
		{
			name: "successfully returns containers missing tags (dummy policy)",
			args: args{
				pods: []v1.Pod{
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "docker.io/kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
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
				},
				ignoreNotRunning:        false,
				missingRegistryOverride: "",
				missingTagPolicy:        "dummy",
				dummyTag:                "UNKNOWN",
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "docker.io/kubernetesui/dashboard:UNKNOWN",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
		{
			name: "successfully returns with an overridden registry",
			args: args{
				pods: []v1.Pod{
					{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "reponame/myimage:0.0.1",
								},
							},
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "test-container",
									Image:       "reponame/myimage:0.0.1",
									ImageID:     "docker.io/reponame/myimage@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
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
									Image: "docker.io/kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac",
								},
							},
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:        "test-container2",
									Image:       "sha256:20b332c9a70d8516d849d1ac23eff5800cbb2f263d379f0ec11ee908db6b25a8",
									ImageID:     "docker-pullable://kubernetesui/dashboard@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
									ContainerID: "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
								},
							},
							Phase: v1.PodRunning,
						},
					},
				},
				ignoreNotRunning:        false,
				missingRegistryOverride: "custom.registry.io",
				missingTagPolicy:        "digest",
				dummyTag:                "",
			},
			want: []Container{
				{
					Name:        "test-container",
					ImageTag:    "custom.registry.io/reponame/myimage:0.0.1",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
				{
					Name:        "test-container2",
					ImageTag:    "docker.io/kubernetesui/dashboard:v2.7.0",
					ImageDigest: "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
					ID:          "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetContainersFromPods(
				tt.args.pods,
				tt.args.ignoreNotRunning,
				tt.args.missingRegistryOverride,
				tt.args.missingTagPolicy,
				tt.args.dummyTag,
			)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getRegistryOverrideNormalisedImageTag(t *testing.T) {
	type args struct {
		imageTag                string
		missingRegistryOverride string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "successfully returns image tag with overridden registry",
			args: args{
				imageTag:                "reponame/myimage:0.0.1",
				missingRegistryOverride: "custom.registry.io",
			},
			want: "custom.registry.io/reponame/myimage:0.0.1",
		},
		{
			name: "successfully returns valid image tag without overridden registry",
			args: args{
				imageTag:                "docker.io/reponame/myimage:0.0.1",
				missingRegistryOverride: "custom.registry.io",
			},
			want: "docker.io/reponame/myimage:0.0.1",
		},
		{
			name: "successfully returns valid image tag without overridden registry (includes library)",
			args: args{
				imageTag:                "docker.io/library/reponame/myimage:0.0.1",
				missingRegistryOverride: "custom.registry.io",
			},
			want: "docker.io/library/reponame/myimage:0.0.1",
		},
		{
			name: "successfully returns valid image tag no repo",
			args: args{
				imageTag:                "myimage:0.0.1",
				missingRegistryOverride: "custom.registry.io",
			},
			want: "custom.registry.io/myimage:0.0.1",
		},
		{
			name: "returns image tag without overridden registry if library and repo are present but no registry (cannot determine between library and domain)",
			args: args{
				imageTag:                "library/reponame/myimage:0.0.1",
				missingRegistryOverride: "custom.registry.io",
			},
			want: "library/reponame/myimage:0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRegistryOverrideNormalisedImageTag(tt.args.imageTag, tt.args.missingRegistryOverride)
			assert.Equal(t, tt.want, got)
		})
	}
}
