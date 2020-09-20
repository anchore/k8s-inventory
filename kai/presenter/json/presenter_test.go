package json

import (
	"bytes"
	"flag"
	"github.com/anchore/kai/kai/result"
	"testing"
	"time"

	"github.com/anchore/go-testutils"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var update = flag.Bool("update", false, "update the *.golden files for json presenters")

func TestJsonPresenter(t *testing.T) {
	var buffer bytes.Buffer

	var namespace1 = result.Namespace{
		Namespace: "docker",
		Images: []string{
			"docker/kube-compose-controller:v0.4.25-alpha1",
			"docker/kube-compose-api-server:v0.4.25-alpha1",
		},
	}

	var namespace2 = result.Namespace{
		Namespace: "kube-system",
		Images: []string{
			"k8s.gcr.io/coredns:1.6.2",
			"k8s.gcr.io/etcd:3.3.15-0",
			"k8s.gcr.io/kube-apiserver:v1.16.5",
			"k8s.gcr.io/kube-controller-manager:v1.16.5",
			"k8s.gcr.io/kube-proxy:v1.16.5",
			"k8s.gcr.io/kube-scheduler:v1.16.5",
			"docker/desktop-storage-provisioner:v1.1",
			"docker/desktop-vpnkit-controller:v1.0",
		},
	}

	var testTime = time.Date(2020, time.September, 18, 11, 00, 49, 0, time.UTC)
	var mockResult = result.Result{
		Timestamp: testTime.Format(time.RFC3339),
		Results:   []result.Namespace{namespace1, namespace2},
	}

	pres := NewPresenter(mockResult)

	// run presenter
	if err := pres.Present(&buffer); err != nil {
		t.Fatal(err)
	}
	actual := buffer.Bytes()
	if *update {
		testutils.UpdateGoldenFileContents(t, actual)
	}

	var expected = testutils.GetGoldenFileContents(t)

	if !bytes.Equal(expected, actual) {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(expected), string(actual), true)
		t.Errorf("mismatched output:\n%s", dmp.DiffPrettyText(diffs))
	}

}

func TestEmptyJsonPresenter(t *testing.T) {
	// Expected to have an empty JSON object back
	var buffer bytes.Buffer

	pres := NewPresenter(result.Result{})

	// run presenter
	err := pres.Present(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	actual := buffer.Bytes()
	if *update {
		testutils.UpdateGoldenFileContents(t, actual)
	}

	var expected = testutils.GetGoldenFileContents(t)

	if !bytes.Equal(expected, actual) {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(expected), string(actual), true)
		t.Errorf("mismatched output:\n%s", dmp.DiffPrettyText(diffs))
	}

}

func TestNoResultsJsonPresenter(t *testing.T) {
	// Expected to have an empty JSON object back
	var buffer bytes.Buffer

	var testTime = time.Date(2020, time.September, 18, 11, 00, 49, 0, time.UTC)
	pres := NewPresenter(result.Result{
		Timestamp: testTime.Format(time.RFC3339),
		Results:   []result.Namespace{},
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

	var expected = testutils.GetGoldenFileContents(t)

	if !bytes.Equal(expected, actual) {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(expected), string(actual), true)
		t.Errorf("mismatched output:\n%s", dmp.DiffPrettyText(diffs))
	}

}
