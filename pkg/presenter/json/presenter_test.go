package json

import (
	"bytes"
	"flag"
	"testing"
	"time"

	"github.com/anchore/k8s-inventory/pkg/inventory"
	"k8s.io/apimachinery/pkg/version"

	"github.com/anchore/go-testutils"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var update = flag.Bool("update", false, "update the *.golden files for json presenters")

func TestJsonPresenter(t *testing.T) {
	var buffer bytes.Buffer

	item1 := inventory.ReportItem{
		Namespace: "docker",
		Images: []inventory.ReportImage{
			{
				Tag:        "docker/kube-compose-controller:v0.4.25-alpha1",
				RepoDigest: "sha256:6ad2d6a2cc1909fbc477f64e3292c16b88db31eb83458f420eb223f119f3dffd",
			},
			{
				Tag:        "docker/kube-compose-api-server:v0.4.25-alpha1",
				RepoDigest: "sha256:17593177ba90d0ece33fa82c0075953df1f29beb89f71c1ee8b13abee31da494",
			},
		},
	}

	item2 := inventory.ReportItem{
		Namespace: "kube-system",
		Images: []inventory.ReportImage{
			{
				Tag:        "k8s.gcr.io/coredns:1.6.2",
				RepoDigest: "sha256:12eb885b8685b1b13a04ecf5c23bc809c2e57917252fd7b0be9e9c00644e8ee5",
			},
			{
				Tag:        "k8s.gcr.io/etcd:3.3.15-0",
				RepoDigest: "sha256:12c2c5e5731c3bcd56e6f1c05c0f9198b6f06793fa7fca2fb43aab9622dc4afa",
			},
			{
				Tag:        "k8s.gcr.io/kube-apiserver:v1.16.5",
				RepoDigest: "sha256:1ec8f8d41f67f3263b86d71f3a7d3d925b2458dd14292baecfbdf18c234a1855",
			},
			{
				Tag:        "k8s.gcr.io/kube-controller-manager:v1.16.5",
				RepoDigest: "sha256:d807554df171ba4f3b56aa2a63c2ef5b56af095fd7aebdeafedbbfcda5275d10",
			},
			{
				Tag:        "k8s.gcr.io/kube-proxy:v1.16.5",
				RepoDigest: "sha256:166939d1b8d0988d675a027f459e40fbded092887905cc1b62b7e4cb67d493c5",
			},
			{
				Tag:        "k8s.gcr.io/kube-scheduler:v1.16.5",
				RepoDigest: "sha256:8f20c90afce972ae51acaf425b7bdb6445f54168b52ea311b2b89adf5db1acac",
			},
			{
				Tag:        "docker/desktop-storage-provisioner:v1.1",
				RepoDigest: "sha256:b39d74c0eb50b82375f916ff2bf0d10cccff09015e01c55eaa123ec6549c4177",
			},
			{
				Tag:        "docker/desktop-vpnkit-controller:v1.0",
				RepoDigest: "sha256:6800d69751e483710a0949fbd01c459934a18ede9d227166def0af44f3a46970",
			},
		},
	}

	testTime := time.Date(2020, time.September, 18, 11, 00, 49, 0, time.UTC)
	mockReport := inventory.Report{
		Timestamp: testTime.Format(time.RFC3339),
		Results:   []inventory.ReportItem{item1, item2},
		ServerVersionMetadata: &version.Info{
			Major:        "1",
			Minor:        "16+",
			GitVersion:   "v1.16.6-beta.0",
			GitCommit:    "e7f962ba86f4ce7033828210ca3556393c377bcc",
			GitTreeState: "clean",
			BuildDate:    "2020-01-15T08:18:29Z",
			GoVersion:    "go1.13.5",
			Compiler:     "gc",
			Platform:     "linux/amd64",
		},
		ClusterName:   "docker-desktop",
		InventoryType: "kubernetes",
	}

	pres := NewPresenter(mockReport)

	// run presenter
	if err := pres.Present(&buffer); err != nil {
		t.Fatal(err)
	}
	actual := buffer.Bytes()
	if *update {
		testutils.UpdateGoldenFileContents(t, actual)
	}

	expected := testutils.GetGoldenFileContents(t)

	if !bytes.Equal(expected, actual) {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(expected), string(actual), true)
		t.Errorf("mismatched output:\n%s", dmp.DiffPrettyText(diffs))
	}
}

func TestEmptyJsonPresenter(t *testing.T) {
	// Expected to have an empty JSON object back
	var buffer bytes.Buffer

	pres := NewPresenter(inventory.Report{})

	// run presenter
	err := pres.Present(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	actual := buffer.Bytes()
	if *update {
		testutils.UpdateGoldenFileContents(t, actual)
	}

	expected := testutils.GetGoldenFileContents(t)

	if !bytes.Equal(expected, actual) {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(expected), string(actual), true)
		t.Errorf("mismatched output:\n%s", dmp.DiffPrettyText(diffs))
	}
}

func TestNoResultsJsonPresenter(t *testing.T) {
	// Expected to have an empty JSON object back
	var buffer bytes.Buffer

	testTime := time.Date(2020, time.September, 18, 11, 00, 49, 0, time.UTC)
	pres := NewPresenter(inventory.Report{
		Timestamp:     testTime.Format(time.RFC3339),
		Results:       []inventory.ReportItem{},
		ClusterName:   "docker-desktop",
		InventoryType: "kubernetes",
	})

	// run presenter
	err := pres.Present(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	actual := buffer.Bytes()
	if *update {
		testutils.UpdateGoldenFileContents(t, actual)
	}

	expected := testutils.GetGoldenFileContents(t)

	if !bytes.Equal(expected, actual) {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(expected), string(actual), true)
		t.Errorf("mismatched output:\n%s", dmp.DiffPrettyText(diffs))
	}
}
