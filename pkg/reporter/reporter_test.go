package reporter

import (
	"testing"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func TestBuildUrl(t *testing.T) {
	anchoreDetails := config.AnchoreInfo{
		URL:      "https://ancho.re",
		User:     "admin",
		Password: "foobar",
	}

	expectedURL := "https://ancho.re/v1/enterprise/kubernetes-inventory"
	actualURL, err := buildURL(anchoreDetails, "v1/enterprise/kubernetes-inventory")
	if err != nil || expectedURL != actualURL {
		t.Errorf("Failed to build URL:\nexpected=%s\nactual=%s", expectedURL, actualURL)
	}

	expectedURL = "https://ancho.re/v2/kubernetes-inventory"
	actualURL, err = buildURL(anchoreDetails, "v2/kubernetes-inventory")
	if err != nil || expectedURL != actualURL {
		t.Errorf("Failed to build URL:\nexpected=%s\nactual=%s", expectedURL, actualURL)
	}
}

func TestPost(t *testing.T) {
	defer gock.Off()

	type args struct {
		report         inventory.Report
		anchoreDetails config.AnchoreInfo
	}
	tests := []struct {
		name            string
		args            args
		wantErr         bool
		expectedAPIPath string
	}{
		{
			name: "default post to v2",
			args: args{
				report: inventory.Report{},
				anchoreDetails: config.AnchoreInfo{
					URL:      "https://ancho.re",
					User:     "admin",
					Password: "foobar",
					Account:  "test",
					HTTP: config.HTTPConfig{
						TimeoutSeconds: 10,
						Insecure:       true,
					},
				},
			},
			wantErr:         false,
			expectedAPIPath: reportAPIPathV2,
		},
		{
			name: "post to v1 when v2 is not found",
			args: args{
				report: inventory.Report{},
				anchoreDetails: config.AnchoreInfo{
					URL:      "https://ancho.re",
					User:     "admin",
					Password: "foobar",
					Account:  "test",
					HTTP: config.HTTPConfig{
						TimeoutSeconds: 10,
						Insecure:       true,
					},
				},
			},
			wantErr:         false,
			expectedAPIPath: reportAPIPathV1,
		},
		{
			name: "error when v1 and v2 are not found",
			args: args{
				report: inventory.Report{},
				anchoreDetails: config.AnchoreInfo{
					URL:      "https://ancho.re",
					User:     "admin",
					Password: "foobar",
					Account:  "test",
					HTTP: config.HTTPConfig{
						TimeoutSeconds: 10,
						Insecure:       true,
					},
				},
			},
			wantErr:         true,
			expectedAPIPath: reportAPIPathV1,
		},
		{
			name: "error when api response is not JSON",
			args: args{
				report: inventory.Report{},
				anchoreDetails: config.AnchoreInfo{
					URL:      "https://ancho.re",
					User:     "admin",
					Password: "foobar",
					Account:  "test",
					HTTP: config.HTTPConfig{
						TimeoutSeconds: 10,
						Insecure:       true,
					},
				},
			},
			wantErr:         true,
			expectedAPIPath: reportAPIPathV2,
		},
	}
	for _, tt := range tests {
		switch tt.name {
		case "default post to v2":
			gock.New("https://ancho.re").
				Post(reportAPIPathV2).
				Reply(201).
				JSON(map[string]interface{}{})
		case "post to v1 when v2 is not found":
			gock.New("https://ancho.re").
				Post(reportAPIPathV2).
				Reply(404)
			gock.New("https://ancho.re").
				Post(reportAPIPathV1).
				Reply(201).
				JSON(map[string]interface{}{})
			gock.New("https://ancho.re").
				Get("/version").
				Reply(200).
				JSON(map[string]interface{}{
					"api":     map[string]interface{}{},
					"db":      map[string]interface{}{"schema_version": "400"},
					"service": map[string]interface{}{"version": "4.8.0"},
				})
		case "error when v1 and v2 are not found":
			gock.New("https://ancho.re").
				Post(reportAPIPathV2).
				Reply(404)
			gock.New("https://ancho.re").
				Get("/version").
				Reply(404)
		case "error when api response is not JSON":
			gock.New("https://ancho.re").
				Post(reportAPIPathV2).
				Reply(201).
				BodyString("not json")
		}

		t.Run(tt.name, func(t *testing.T) {
			// Reset enterpriseEndpoint to the default each test run
			enterpriseEndpoint = reportAPIPathV2

			err := Post(tt.args.report, tt.args.anchoreDetails)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAPIPath, enterpriseEndpoint)
			}
		})
	}
}

// Simulate a handover from Enterprise 4.x to 5.x
// In this case v1 should be used initially instead of v2 then when v1 is no longer available v2 should be used
func TestPostSimulateV1ToV2HandoverFromEnterprise4Xto5X(t *testing.T) {
	defer gock.Off()

	testReport := inventory.Report{}
	testAnchoreDetails := config.AnchoreInfo{
		URL:      "https://ancho.re",
		User:     "admin",
		Password: "foobar",
		Account:  "test",
		HTTP: config.HTTPConfig{
			TimeoutSeconds: 10,
			Insecure:       true,
		},
	}

	enterpriseEndpoint = reportAPIPathV2

	// After the first post to default v2, the enterpriseEndpoint should be set to v1
	gock.New("https://ancho.re").
		Post(reportAPIPathV2).
		Reply(404)
	gock.New("https://ancho.re").
		Get("/version").
		Reply(200).
		JSON(map[string]interface{}{
			"api":     map[string]interface{}{},
			"db":      map[string]interface{}{"schema_version": "400"},
			"service": map[string]interface{}{"version": "4.8.0"},
		})
	gock.New("https://ancho.re").
		Post(reportAPIPathV1).
		Reply(201).
		JSON(map[string]interface{}{})
	err := Post(testReport, testAnchoreDetails)
	assert.NoError(t, err)
	assert.Equal(t, reportAPIPathV1, enterpriseEndpoint)

	// Simulate upgrade to Enterprise 5.x, v1 should no longer be available
	gock.New("https://ancho.re").
		Post(reportAPIPathV1).
		Reply(404)
	gock.New("https://ancho.re").
		Get("/version").
		Reply(200).
		JSON(map[string]interface{}{
			"api":     map[string]interface{}{"version": "2"},
			"db":      map[string]interface{}{"schema_version": "400"},
			"service": map[string]interface{}{"version": "4.8.0"},
		})
	gock.New("https://ancho.re").
		Post(reportAPIPathV2).
		Reply(201).
		JSON(map[string]interface{}{})
	err = Post(testReport, testAnchoreDetails)
	assert.NoError(t, err)
	assert.Equal(t, reportAPIPathV2, enterpriseEndpoint)
}

func TestNormalize(t *testing.T) {
	type args struct {
		report inventory.Report
	}
	tests := []struct {
		name     string
		args     args
		want     inventory.Report
		modified bool
	}{
		{
			name: "no modifications",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							ID:          "cont1",
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace",
							UID:  "ns1",
						},
					},
					Nodes: []inventory.Node{
						{
							Name: "testNode",
							UID:  "node1",
						},
					},
					Pods: []inventory.Pod{
						{
							Name:         "testPod",
							NamespaceUID: "ns1",
							UID:          "pod1",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont1",
						Name:        "testContainer",
						PodUID:      "pod1",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace",
						UID:  "ns1",
					},
				},
				Nodes: []inventory.Node{
					{
						Name: "testNode",
						UID:  "node1",
					},
				},
				Pods: []inventory.Pod{
					{
						Name:         "testPod",
						NamespaceUID: "ns1",
						UID:          "pod1",
						NodeUID:      "node1",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: false,
		},
		{
			name: "namespace missing UID",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							ID:          "cont1",
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
						{
							ID:          "cont2",
							Name:        "testContainer2",
							PodUID:      "pod2",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace",
							UID:  "",
						},
						{
							Name: "testNamespace2",
							UID:  "ns2",
						},
					},
					Nodes: []inventory.Node{
						{
							Name: "testNode",
							UID:  "node1",
						},
					},
					Pods: []inventory.Pod{
						{
							Name:         "testPod",
							NamespaceUID: "ns1",
							UID:          "pod1",
							NodeUID:      "node1",
						},
						{
							Name:         "testPod2",
							NamespaceUID: "ns2",
							UID:          "pod2",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont2",
						Name:        "testContainer2",
						PodUID:      "pod2",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace2",
						UID:  "ns2",
					},
				},
				Nodes: []inventory.Node{
					{
						Name: "testNode",
						UID:  "node1",
					},
				},
				Pods: []inventory.Pod{
					{
						Name:         "testPod2",
						NamespaceUID: "ns2",
						UID:          "pod2",
						NodeUID:      "node1",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: true,
		},
		{
			name: "namespace missing",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							ID:          "cont1",
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
						{
							ID:          "cont2",
							Name:        "testContainer2",
							PodUID:      "pod2",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace2",
							UID:  "ns2",
						},
					},
					Nodes: []inventory.Node{
						{
							Name: "testNode",
							UID:  "node1",
						},
					},
					Pods: []inventory.Pod{
						{
							Name:         "testPod",
							NamespaceUID: "ns1",
							UID:          "pod1",
							NodeUID:      "node1",
						},
						{
							Name:         "testPod2",
							NamespaceUID: "ns2",
							UID:          "pod2",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont2",
						Name:        "testContainer2",
						PodUID:      "pod2",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace2",
						UID:  "ns2",
					},
				},
				Nodes: []inventory.Node{
					{
						Name: "testNode",
						UID:  "node1",
					},
				},
				Pods: []inventory.Pod{
					{
						Name:         "testPod2",
						NamespaceUID: "ns2",
						UID:          "pod2",
						NodeUID:      "node1",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: true,
		},
		{
			name: "node missing UID",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							ID:          "cont1",
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
						{
							ID:          "cont2",
							Name:        "testContainer2",
							PodUID:      "pod2",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace",
							UID:  "ns1",
						},
						{
							Name: "testNamespace2",
							UID:  "ns2",
						},
					},
					Nodes: []inventory.Node{
						{
							Name: "testNode",
							UID:  "",
						},
					},
					Pods: []inventory.Pod{
						{
							Name:         "testPod",
							NamespaceUID: "ns1",
							UID:          "pod1",
							NodeUID:      "node1",
						},
						{
							Name:         "testPod2",
							NamespaceUID: "ns2",
							UID:          "pod2",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont1",
						Name:        "testContainer",
						PodUID:      "pod1",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
					{
						ID:          "cont2",
						Name:        "testContainer2",
						PodUID:      "pod2",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace",
						UID:  "ns1",
					},
					{
						Name: "testNamespace2",
						UID:  "ns2",
					},
				},
				Nodes: []inventory.Node{},
				Pods: []inventory.Pod{
					{
						Name:         "testPod",
						NamespaceUID: "ns1",
						UID:          "pod1",
					},
					{
						Name:         "testPod2",
						NamespaceUID: "ns2",
						UID:          "pod2",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: true,
		},
		{
			name: "node missing",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							ID:          "cont1",
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
						{
							ID:          "cont2",
							Name:        "testContainer2",
							PodUID:      "pod2",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace",
							UID:  "ns1",
						},
						{
							Name: "testNamespace2",
							UID:  "ns2",
						},
					},
					Nodes: []inventory.Node{},
					Pods: []inventory.Pod{
						{
							Name:         "testPod",
							NamespaceUID: "ns1",
							UID:          "pod1",
							NodeUID:      "node1",
						},
						{
							Name:         "testPod2",
							NamespaceUID: "ns2",
							UID:          "pod2",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont1",
						Name:        "testContainer",
						PodUID:      "pod1",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
					{
						ID:          "cont2",
						Name:        "testContainer2",
						PodUID:      "pod2",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace",
						UID:  "ns1",
					},
					{
						Name: "testNamespace2",
						UID:  "ns2",
					},
				},
				Nodes: []inventory.Node{},
				Pods: []inventory.Pod{
					{
						Name:         "testPod",
						NamespaceUID: "ns1",
						UID:          "pod1",
					},
					{
						Name:         "testPod2",
						NamespaceUID: "ns2",
						UID:          "pod2",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: true,
		},
		{
			name: "container missing UID",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
						{
							ID:          "cont2",
							Name:        "testContainer2",
							PodUID:      "pod2",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace",
							UID:  "ns1",
						},
						{
							Name: "testNamespace2",
							UID:  "ns2",
						},
					},
					Nodes: []inventory.Node{
						{
							Name: "testNode",
							UID:  "node1",
						},
					},
					Pods: []inventory.Pod{
						{
							Name:         "testPod",
							NamespaceUID: "ns1",
							UID:          "pod1",
							NodeUID:      "node1",
						},
						{
							Name:         "testPod2",
							NamespaceUID: "ns2",
							UID:          "pod2",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont2",
						Name:        "testContainer2",
						PodUID:      "pod2",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace",
						UID:  "ns1",
					},
					{
						Name: "testNamespace2",
						UID:  "ns2",
					},
				},
				Nodes: []inventory.Node{
					{
						Name: "testNode",
						UID:  "node1",
					},
				},
				Pods: []inventory.Pod{
					{
						Name:         "testPod",
						NamespaceUID: "ns1",
						UID:          "pod1",
						NodeUID:      "node1",
					},
					{
						Name:         "testPod2",
						NamespaceUID: "ns2",
						UID:          "pod2",
						NodeUID:      "node1",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: true,
		},
		{
			name: "pod missing UID",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							ID:          "cont1",
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
						{
							ID:          "cont2",
							Name:        "testContainer2",
							PodUID:      "pod2",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace",
							UID:  "ns1",
						},
						{
							Name: "testNamespace2",
							UID:  "ns2",
						},
					},
					Nodes: []inventory.Node{
						{
							Name: "testNode",
							UID:  "node1",
						},
					},
					Pods: []inventory.Pod{
						{
							Name:         "testPod",
							NamespaceUID: "ns1",
							NodeUID:      "node1",
						},
						{
							Name:         "testPod2",
							NamespaceUID: "ns2",
							UID:          "pod2",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont2",
						Name:        "testContainer2",
						PodUID:      "pod2",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace",
						UID:  "ns1",
					},
					{
						Name: "testNamespace2",
						UID:  "ns2",
					},
				},
				Nodes: []inventory.Node{
					{
						Name: "testNode",
						UID:  "node1",
					},
				},
				Pods: []inventory.Pod{
					{
						Name:         "testPod2",
						NamespaceUID: "ns2",
						UID:          "pod2",
						NodeUID:      "node1",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: true,
		},
		{
			name: "pod missing",
			args: args{
				report: inventory.Report{
					ClusterName: "test",
					Containers: []inventory.Container{
						{
							ID:          "cont1",
							Name:        "testContainer",
							PodUID:      "pod1",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
						{
							ID:          "cont2",
							Name:        "testContainer2",
							PodUID:      "pod2",
							ImageDigest: "sha256:123456",
							ImageTag:    "myimage:latest",
						},
					},
					Namespaces: []inventory.Namespace{
						{
							Name: "testNamespace",
							UID:  "ns1",
						},
						{
							Name: "testNamespace2",
							UID:  "ns2",
						},
					},
					Nodes: []inventory.Node{
						{
							Name: "testNode",
							UID:  "node1",
						},
					},
					Pods: []inventory.Pod{
						{
							Name:         "testPod2",
							NamespaceUID: "ns2",
							UID:          "pod2",
							NodeUID:      "node1",
						},
					},
					ServerVersionMetadata: nil,
					Timestamp:             "2021-01-01T00:00:00Z",
				},
			},
			want: inventory.Report{
				ClusterName: "test",
				Containers: []inventory.Container{
					{
						ID:          "cont2",
						Name:        "testContainer2",
						PodUID:      "pod2",
						ImageDigest: "sha256:123456",
						ImageTag:    "myimage:latest",
					},
				},
				Namespaces: []inventory.Namespace{
					{
						Name: "testNamespace",
						UID:  "ns1",
					},
					{
						Name: "testNamespace2",
						UID:  "ns2",
					},
				},
				Nodes: []inventory.Node{
					{
						Name: "testNode",
						UID:  "node1",
					},
				},
				Pods: []inventory.Pod{
					{
						Name:         "testPod2",
						NamespaceUID: "ns2",
						UID:          "pod2",
						NodeUID:      "node1",
					},
				},
				ServerVersionMetadata: nil,
				Timestamp:             "2021-01-01T00:00:00Z",
			},
			modified: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, mod := Normalize(tt.args.report)
			assert.Equal(t, tt.want, report)
			assert.Equal(t, tt.modified, mod)
		})
	}
}
