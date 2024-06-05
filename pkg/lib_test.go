package pkg

import (
	"sort"
	"testing"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/stretchr/testify/assert"
)

var (
	TestNamespace1 = inventory.Namespace{
		Name: "ns1",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"anchore.io/account": "account1",
		},
		UID: "ns1_UID",
	}
	TestNamespace2 = inventory.Namespace{
		Name: "ns2",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"anchore.io/account": "account2",
		},
		UID: "ns2_UID",
	}
	TestNamespace3 = inventory.Namespace{
		Name: "ns3",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"anchore.io/account": "account3",
		},
		UID: "ns3_UID",
	}
	TestNamespace4 = inventory.Namespace{
		Name: "ns4",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"anchore.io/account": "account4",
		},
		UID: "ns4_UID",
	}
	TestNamespace5 = inventory.Namespace{
		Name: "ns5-no-label",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{},
		UID:    "ns5_UID",
	}
	TestNamespaces = []inventory.Namespace{
		TestNamespace1,
		TestNamespace2,
		TestNamespace3,
		TestNamespace4,
	}
)

func TestGetAccountRoutedNamespaces(t *testing.T) {
	type args struct {
		defaultAccount        string
		namespaces            []inventory.Namespace
		accountRoutes         config.AccountRoutes
		namespaceLabelRouting config.AccountRouteByNamespaceLabel
	}
	tests := []struct {
		name string
		args args
		want map[string][]inventory.Namespace
	}{
		{
			name: "no account routes all to default",
			args: args{
				defaultAccount:        "admin",
				namespaces:            TestNamespaces,
				accountRoutes:         config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{},
			},
			want: map[string][]inventory.Namespace{
				"admin": TestNamespaces,
			},
		},
		{
			name: "namespaces to individual accounts explicit",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes: config.AccountRoutes{
					"account1": config.AccountRouteDetails{
						Namespaces: []string{"ns1"},
					},
					"account2": config.AccountRouteDetails{
						Namespaces: []string{"ns2"},
					},
					"account3": config.AccountRouteDetails{
						Namespaces: []string{"ns3"},
					},
					"account4": config.AccountRouteDetails{
						Namespaces: []string{"ns4"},
					},
				},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{},
			},
			want: map[string][]inventory.Namespace{
				"account1": {TestNamespace1},
				"account2": {TestNamespace2},
				"account3": {TestNamespace3},
				"account4": {TestNamespace4},
			},
		},
		{
			name: "namespaces to account regex",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes: config.AccountRoutes{
					"account1": config.AccountRouteDetails{
						Namespaces: []string{"ns.*"},
					},
				},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{},
			},
			want: map[string][]inventory.Namespace{
				"account1": TestNamespaces,
			},
		},
		{
			name: "namespaces to accounts that match a label only",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "default",
					IgnoreMissingLabel: false,
				},
			},
			want: map[string][]inventory.Namespace{
				"account1": {TestNamespace1},
				"account2": {TestNamespace2},
				"account3": {TestNamespace3},
				"account4": {TestNamespace4},
			},
		},
		{
			name: "namespaces to accounts that match a label only with namespace missing label (default account not set)",
			args: args{
				defaultAccount: "admin",
				namespaces:     append(TestNamespaces, TestNamespace5),
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "",
					IgnoreMissingLabel: false,
				},
			},
			want: map[string][]inventory.Namespace{
				"account1": {TestNamespace1},
				"account2": {TestNamespace2},
				"account3": {TestNamespace3},
				"account4": {TestNamespace4},
				"admin":    {TestNamespace5},
			},
		},
		{
			name: "namespaces to accounts that match a label only with namespace missing label (default account set)",
			args: args{
				defaultAccount: "admin",
				namespaces:     append(TestNamespaces, TestNamespace5),
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "defaultoverride",
					IgnoreMissingLabel: false,
				},
			},
			want: map[string][]inventory.Namespace{
				"account1":        {TestNamespace1},
				"account2":        {TestNamespace2},
				"account3":        {TestNamespace3},
				"account4":        {TestNamespace4},
				"defaultoverride": {TestNamespace5},
			},
		},
		{
			name: "namespaces to accounts that match a label only with namespace missing label set to ignore",
			args: args{
				defaultAccount: "admin",
				namespaces:     append(TestNamespaces, TestNamespace5),
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "",
					IgnoreMissingLabel: true,
				},
			},
			want: map[string][]inventory.Namespace{
				"account1": {TestNamespace1},
				"account2": {TestNamespace2},
				"account3": {TestNamespace3},
				"account4": {TestNamespace4},
			},
		},
		{
			name: "mix of account routes and label routing",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes: config.AccountRoutes{
					"explicitaccount1": config.AccountRouteDetails{
						Namespaces: []string{"ns1"},
					},
				},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "default",
					IgnoreMissingLabel: false,
				},
			},
			want: map[string][]inventory.Namespace{
				"explicitaccount1": {TestNamespace1},
				"account2":         {TestNamespace2},
				"account3":         {TestNamespace3},
				"account4":         {TestNamespace4},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAccountRoutedNamespaces(tt.args.defaultAccount, tt.args.namespaces, tt.args.accountRoutes, tt.args.namespaceLabelRouting)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetNamespacesBatches(t *testing.T) {
	type args struct {
		namespaces []inventory.Namespace
		batchSize  int
	}
	tests := []struct {
		name string
		args args
		want [][]inventory.Namespace
	}{
		{
			name: "empty namespaces",
			args: args{
				namespaces: []inventory.Namespace{},
				batchSize:  10,
			},
			want: [][]inventory.Namespace{},
		},
		{
			name: "single batch",
			args: args{
				namespaces: TestNamespaces,
				batchSize:  10,
			},
			want: [][]inventory.Namespace{
				TestNamespaces,
			},
		},
		{
			name: "multiple batches",
			args: args{
				namespaces: TestNamespaces,
				batchSize:  2,
			},
			want: [][]inventory.Namespace{
				{TestNamespace1, TestNamespace2},
				{TestNamespace3, TestNamespace4},
			},
		},
		{
			name: "multiple batches with remainder",
			args: args{
				namespaces: append(TestNamespaces, TestNamespace5),
				batchSize:  2,
			},
			want: [][]inventory.Namespace{
				{TestNamespace1, TestNamespace2},
				{TestNamespace3, TestNamespace4},
				{TestNamespace5},
			},
		},
		{
			name: "no batches configured (batch size 0)",
			args: args{
				namespaces: TestNamespaces,
				batchSize:  0,
			},
			want: [][]inventory.Namespace{
				TestNamespaces,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetNamespacesBatches(tt.args.namespaces, tt.args.batchSize)
			assert.Equal(t, tt.want, got)
		})
	}
}

var (
	Container1 = inventory.Container{
		Name:        "container1",
		ID:          "container1_ID",
		ImageDigest: "sha256:1234567890",
		ImageTag:    "latest",
		PodUID:      "pod1_UID",
	}
	Container2 = inventory.Container{
		Name:        "container2",
		ID:          "container2_ID",
		ImageDigest: "sha256:1234567890",
		ImageTag:    "latest",
		PodUID:      "pod2_UID",
	}
	Container3 = inventory.Container{
		Name:        "container3",
		ID:          "container3_ID",
		ImageDigest: "sha256:1234567890",
		ImageTag:    "latest",
		PodUID:      "pod3_UID",
	}
	Container4 = inventory.Container{
		Name:        "container4",
		ID:          "container4_ID",
		ImageDigest: "sha256:1234567890",
		ImageTag:    "latest",
		PodUID:      "pod4_UID",
	}
	Container5 = inventory.Container{
		Name:        "container5",
		ID:          "container5_ID",
		ImageDigest: "sha256:1234567890",
		ImageTag:    "latest",
		PodUID:      "pod5_UID",
	}
	Pod1 = inventory.Pod{
		Name:         "pod1",
		NamespaceUID: "ns1_UID",
		UID:          "pod1_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"app": "myapp",
			"env": "dev",
		},
		NodeUID: "node1_UID",
	}
	Pod2 = inventory.Pod{
		Name:         "pod2",
		NamespaceUID: "ns2_UID",
		UID:          "pod2_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"app": "myapp",
			"env": "dev",
		},
		NodeUID: "node2_UID",
	}
	Pod3 = inventory.Pod{
		Name:         "pod3",
		NamespaceUID: "ns3_UID",
		UID:          "pod3_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"app": "myapp",
			"env": "dev",
		},
		NodeUID: "node2_UID",
	}
	Pod4 = inventory.Pod{
		Name:         "pod4",
		NamespaceUID: "ns4_UID",
		UID:          "pod4_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"app": "myapp",
			"env": "dev",
		},
		NodeUID: "node3_UID",
	}
	Pod5 = inventory.Pod{
		Name:         "pod5",
		NamespaceUID: "ns5_UID",
		UID:          "pod5_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"app": "myapp",
			"env": "dev",
		},
		NodeUID: "node3_UID",
	}
	Node1 = inventory.Node{
		Name: "node1",
		UID:  "node1_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"node-role.kubernetes.io/master":   "",
			"node.kubernetes.io/instance-type": "m5.large",
		},
		Arch:                    "amd64",
		ContainerRuntimeVersion: "docker://19.3.1",
		KernelVersion:           "4.19.76",
		KubeProxyVersion:        "v1.17.3",
		KubeletVersion:          "v1.17.3",
		OperatingSystem:         "linux",
	}
	Node2 = inventory.Node{
		Name: "node2",
		UID:  "node2_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"node-role.kubernetes.io/master":   "",
			"node.kubernetes.io/instance-type": "m5.large",
		},
		Arch:                    "amd64",
		ContainerRuntimeVersion: "docker://19.3.1",
		KernelVersion:           "4.19.76",
		KubeProxyVersion:        "v1.17.3",
		KubeletVersion:          "v1.17.3",
		OperatingSystem:         "linux",
	}
	Node3 = inventory.Node{
		Name: "node3",
		UID:  "node3_UID",
		Annotations: map[string]string{
			"anchore.io/account": "account1",
			"anchore.io/cluster": "cluster1",
		},
		Labels: map[string]string{
			"node-role.kubernetes.io/master":   "",
			"node.kubernetes.io/instance-type": "m5.large",
		},
		Arch:                    "amd64",
		ContainerRuntimeVersion: "docker://19.3.1",
		KernelVersion:           "4.19.76",
		KubeProxyVersion:        "v1.17.3",
		KubeletVersion:          "v1.17.3",
		OperatingSystem:         "linux",
	}
	TestReport = inventory.Report{
		ClusterName: "cluster1",
		Namespaces: []inventory.Namespace{
			TestNamespace1,
			TestNamespace2,
			TestNamespace3,
			TestNamespace4,
			TestNamespace5,
		},
		Containers: []inventory.Container{
			Container1,
			Container2,
			Container3,
			Container4,
			Container5,
		},
		Pods: []inventory.Pod{
			Pod1,
			Pod2,
			Pod3,
			Pod4,
			Pod5,
		},
		Nodes: []inventory.Node{
			Node1,
			Node2,
			Node3,
		},
	}
)

func Test_getBatchedInventoryReports(t *testing.T) {
	type args struct {
		reports   AccountRoutedReports
		batchSize int
	}
	tests := []struct {
		name string
		args args
		want BatchedReports
	}{
		{
			name: "empty reports",
			args: args{
				reports:   AccountRoutedReports{},
				batchSize: 10,
			},
			want: BatchedReports{},
		},
		{
			name: "no batches configured (batch size 0)",
			args: args{
				reports: AccountRoutedReports{
					"account1": TestReport,
				},
				batchSize: 0,
			},
			want: BatchedReports{
				"account1": {
					TestReport,
				},
			},
		},
		{
			name: "single batch (namespace count < batch size)",
			args: args{
				reports: AccountRoutedReports{
					"account1": TestReport,
				},
				batchSize: 10,
			},
			want: BatchedReports{
				"account1": {
					TestReport,
				},
			},
		},
		{
			name: "single batch (namespace count == batch size)",
			args: args{
				reports: AccountRoutedReports{
					"account1": TestReport,
				},
				batchSize: 5,
			},
			want: BatchedReports{
				"account1": {
					TestReport,
				},
			},
		},
		{
			name: "multiple batches",
			args: args{
				reports: AccountRoutedReports{
					"account1": TestReport,
				},
				batchSize: 1,
			},
			want: BatchedReports{
				"account1": {
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace1},
						Containers:  []inventory.Container{Container1},
						Pods:        []inventory.Pod{Pod1},
						Nodes:       []inventory.Node{Node1},
					},
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace2},
						Containers:  []inventory.Container{Container2},
						Pods:        []inventory.Pod{Pod2},
						Nodes:       []inventory.Node{Node2},
					},
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace3},
						Containers:  []inventory.Container{Container3},
						Pods:        []inventory.Pod{Pod3},
						Nodes:       []inventory.Node{Node2},
					},
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace4},
						Containers:  []inventory.Container{Container4},
						Pods:        []inventory.Pod{Pod4},
						Nodes:       []inventory.Node{Node3},
					},
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace5},
						Containers:  []inventory.Container{Container5},
						Pods:        []inventory.Pod{Pod5},
						Nodes:       []inventory.Node{Node3},
					},
				},
			},
		},
		{
			name: "multiple batches (2 expected)",
			args: args{
				reports: AccountRoutedReports{
					"account1": TestReport,
				},
				batchSize: 3,
			},
			want: BatchedReports{
				"account1": {
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace1, TestNamespace2, TestNamespace3},
						Containers:  []inventory.Container{Container1, Container2, Container3},
						Pods:        []inventory.Pod{Pod1, Pod2, Pod3},
						Nodes:       []inventory.Node{Node1, Node2},
					},
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace4, TestNamespace5},
						Containers:  []inventory.Container{Container4, Container5},
						Pods:        []inventory.Pod{Pod4, Pod5},
						Nodes:       []inventory.Node{Node3},
					},
				},
			},
		},
		{
			name: "multiple batches (2 expected) x 2 accounts",
			args: args{
				reports: AccountRoutedReports{
					"account1": TestReport,
					"account2": TestReport,
				},
				batchSize: 3,
			},
			want: BatchedReports{
				"account1": {
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace1, TestNamespace2, TestNamespace3},
						Containers:  []inventory.Container{Container1, Container2, Container3},
						Pods:        []inventory.Pod{Pod1, Pod2, Pod3},
						Nodes:       []inventory.Node{Node1, Node2},
					},
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace4, TestNamespace5},
						Containers:  []inventory.Container{Container4, Container5},
						Pods:        []inventory.Pod{Pod4, Pod5},
						Nodes:       []inventory.Node{Node3},
					},
				},
				"account2": {
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace1, TestNamespace2, TestNamespace3},
						Containers:  []inventory.Container{Container1, Container2, Container3},
						Pods:        []inventory.Pod{Pod1, Pod2, Pod3},
						Nodes:       []inventory.Node{Node1, Node2},
					},
					{
						ClusterName: "cluster1",
						Namespaces:  []inventory.Namespace{TestNamespace4, TestNamespace5},
						Containers:  []inventory.Container{Container4, Container5},
						Pods:        []inventory.Pod{Pod4, Pod5},
						Nodes:       []inventory.Node{Node3},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBatchedInventoryReports(tt.args.reports, tt.args.batchSize)
			// Sort the reports for comparison
			for _, reports := range got {
				for _, inner := range reports {
					sort.Slice(inner.Namespaces, func(i, j int) bool {
						return inner.Namespaces[i].Name < inner.Namespaces[j].Name
					})
					sort.Slice(inner.Containers, func(i, j int) bool {
						return inner.Containers[i].Name < inner.Containers[j].Name
					})
					sort.Slice(inner.Pods, func(i, j int) bool {
						return inner.Pods[i].Name < inner.Pods[j].Name
					})
					sort.Slice(inner.Nodes, func(i, j int) bool {
						return inner.Nodes[i].Name < inner.Nodes[j].Name
					})
				}
			}
			for _, reports := range tt.want {
				for _, inner := range reports {
					sort.Slice(inner.Namespaces, func(i, j int) bool {
						return inner.Namespaces[i].Name < inner.Namespaces[j].Name
					})
					sort.Slice(inner.Containers, func(i, j int) bool {
						return inner.Containers[i].Name < inner.Containers[j].Name
					})
					sort.Slice(inner.Pods, func(i, j int) bool {
						return inner.Pods[i].Name < inner.Pods[j].Name
					})
					sort.Slice(inner.Nodes, func(i, j int) bool {
						return inner.Nodes[i].Name < inner.Nodes[j].Name
					})
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
