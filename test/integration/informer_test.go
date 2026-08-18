package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/anchore/k8s-inventory/cmd"
	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg"
	"github.com/anchore/k8s-inventory/pkg/client"
	"github.com/anchore/k8s-inventory/pkg/watcher"
)

const (
	transientPodName  = "k8s-inventory-transient-pod"
	transientPodImage = "busybox:1.36"
	// generous enough that a watch event for a resource the api-server has
	// already deleted has certainly been delivered to the informer
	informerSettleTime = 5 * time.Second
)

func transientPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      transientPodName,
			Namespace: IntegrationTestNamespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "transient",
					Image:   transientPodImage,
					Command: []string{"sleep", "300"},
				},
			},
		},
	}
}

func containerImageTags(reports pkg.BatchedReports) []string {
	tags := make([]string, 0)
	for _, reportsForAccount := range reports {
		for _, report := range reportsForAccount {
			for _, container := range report.Containers {
				tags = append(tags, container.ImageTag)
			}
		}
	}
	return tags
}

// TestInformerCapturesTransientPod checks that a pod which is created and
// deleted in between two inventory reports is still reported, which is the
// behaviour the polling collection method cannot provide.
//
// Assumes that the hello-world helm chart in ./fixtures was installed, which is
// what creates the integration test namespace.
func TestInformerCapturesTransientPod(t *testing.T) {
	cmd.InitAppConfig()
	cfg := cmd.GetAppConfig()
	require.Equal(t, config.InventoryCollectionMethodInformer, cfg.InventoryCollection.Method,
		"the informer collection method should be the default")

	kubeconfig, err := client.GetKubeConfig(cfg)
	require.NoError(t, err)
	clientset, err := client.GetClientSet(kubeconfig)
	require.NoError(t, err)

	w, err := watcher.New(clientset, watcher.Config{
		RequestTimeout:   30 * time.Second,
		CacheSyncTimeout: 60 * time.Second,
	})
	require.NoError(t, err)

	stopCh := make(chan struct{})
	defer close(stopCh)
	require.NoError(t, w.Start(stopCh))

	serverVersion, err := clientset.Discovery().ServerVersion()
	require.NoError(t, err)

	// the initial report is the equivalent of the first tick after startup
	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	require.NotContains(t, containerImageTags(pkg.GetInventoryReportsFromSnapshot(cfg, snapshot, serverVersion)), transientPodImage)

	pods := clientset.CoreV1().Pods(IntegrationTestNamespace)
	_, err = pods.Create(context.Background(), transientPod(), metav1.CreateOptions{})
	require.NoError(t, err)
	defer func() {
		_ = pods.Delete(context.Background(), transientPodName, metav1.DeleteOptions{})
	}()

	// the pod comes...
	require.Eventually(t, func() bool {
		pod, err := pods.Get(context.Background(), transientPodName, metav1.GetOptions{})
		return err == nil && pod.Status.Phase == corev1.PodRunning
	}, 2*time.Minute, time.Second, "transient pod never reached the Running phase")

	// ...and goes, all without a report being generated
	gracePeriod := int64(0)
	require.NoError(t, pods.Delete(context.Background(), transientPodName,
		metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}))
	require.Eventually(t, func() bool {
		_, err := pods.Get(context.Background(), transientPodName, metav1.GetOptions{})
		return k8sErrors.IsNotFound(err)
	}, time.Minute, time.Second, "transient pod was never removed")
	time.Sleep(informerSettleTime)

	// the next report still contains the container that briefly existed
	snapshot, err = w.Snapshot()
	require.NoError(t, err)
	assert.Contains(t, containerImageTags(pkg.GetInventoryReportsFromSnapshot(cfg, snapshot, serverVersion)), transientPodImage,
		"the transient pod's container was not reported")

	// and the report after that does not, as the pod is long gone
	snapshot, err = w.Snapshot()
	require.NoError(t, err)
	assert.NotContains(t, containerImageTags(pkg.GetInventoryReportsFromSnapshot(cfg, snapshot, serverVersion)), transientPodImage,
		"the transient pod's container was reported more than once")
}
