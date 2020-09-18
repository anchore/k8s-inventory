package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/anchore/kai/internal"
	"github.com/anchore/kai/internal/bus"
	"github.com/anchore/kai/internal/ui"
	"github.com/anchore/kai/internal/version"
	"github.com/anchore/kai/kai/event"
	"github.com/anchore/kai/kai/presenter"
	"github.com/anchore/kai/kai/result"
	"github.com/spf13/cobra"
	"github.com/wagoodman/go-partybus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kai",
	Short: "KAI tells Anchore which images are in use in your Kubernetes Cluster",
	Long: `KAI (Kubernetes Automated Inventory) can be configured to either poll or watch (using SharedInformers) a 
    Kubernetes Cluster to tell Anchore which Images are currently in-use`,
	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if appConfig.Dev.ProfileCPU {
			f, err := os.Create("cpu.profile")
			if err != nil {
				log.Errorf("unable to create CPU profile: %+v", err)
			} else {
				err := pprof.StartCPUProfile(f)
				if err != nil {
					log.Errorf("unable to start CPU profile: %+v", err)
				}
			}
		}
		if len(args) > 0 {
			err := cmd.Help()
			if err != nil {
				log.Errorf(err.Error())
				os.Exit(1)
			}
			os.Exit(1)
		}
		err := runDefaultCmd()

		if appConfig.Dev.ProfileCPU {
			pprof.StopCPUProfile()
		}

		if err != nil {
			log.Errorf(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	// output & formatting options
	opt := "output"
	rootCmd.Flags().StringP(
		opt, "o", presenter.JSONPresenter.String(),
		fmt.Sprintf("report output formatter, options=%v", presenter.Options),
	)
	if err := viper.BindPFlag(opt, rootCmd.Flags().Lookup(opt)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", opt, err)
		os.Exit(1)
	}

	opt = "kubeconfig"
	home := homeDir()
	rootCmd.Flags().StringP(opt, "k", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	if err := viper.BindPFlag(opt, rootCmd.Flags().Lookup(opt)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", opt, err)
		os.Exit(1)
	}
}

//nolint:funlen
func getImageResults() <-chan error {
	errs := make(chan error)
	go func() {
		defer close(errs)

		if appConfig.CheckForAppUpdate {
			isAvailable, newVersion, err := version.IsUpdateAvailable()
			if err != nil {
				log.Errorf(err.Error())
			}
			if isAvailable {
				log.Infof("New version of %s is available: %s", internal.ApplicationName, newVersion)

				bus.Publish(partybus.Event{
					Type:  event.AppUpdateAvailable,
					Value: newVersion,
				})
			} else {
				log.Debugf("No new %s update available", internal.ApplicationName)
			}
		}

		// use the current context in kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", appConfig.KubeConfig)
		if err != nil {
			errs <- fmt.Errorf("failed to build kube client config: %w", err)
		}

		// create the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			errs <- fmt.Errorf("failed to build kube clientset: %w", err)
		}

		pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			errs <- fmt.Errorf("failed to List Pods: %w", err)
		}
		log.Debugf("There are %d pods in the cluster\n", len(pods.Items))

		namespaceMap := make(map[string]*result.Namespace)
		for _, pod := range pods.Items {
			namespace := pod.ObjectMeta.Namespace
			if namespace == "" || len(pod.Spec.Containers) == 0 {
				continue
			}

			if value, ok := namespaceMap[namespace]; ok {
				value.AddImages(getUniqueImagesFromPodSpec(pod.Spec.Containers))
			} else {
				imageList := getUniqueImagesFromPodSpec(pod.Spec.Containers)
				namespaceMap[namespace] = &result.Namespace{
					Name:   namespace,
					Images: imageList,
				}
			}
		}

		namespaces := make([]result.Namespace, 0)
		for _, value := range namespaceMap {
			namespaces = append(namespaces, *value)
		}
		imagesResult := result.Result{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Results:   namespaces,
		}
		bus.Publish(partybus.Event{
			Type:  event.ImageResultsRetrieved,
			Value: presenter.GetPresenter(appConfig.PresenterOpt, imagesResult),
		})
	}()
	return errs
}

func runDefaultCmd() error {
	errs := getImageResults()
	ux := ui.Select(appConfig.CliOptions.Verbosity > 0, appConfig.Quiet)
	return ux(errs, eventSubscription)
}

func getUniqueImagesFromPodSpec(containers []v1.Container) []string {
	imageMap := make(map[string]struct{})
	for _, container := range containers {
		imageMap[container.Image] = struct{}{}
	}
	imageSlice := make([]string, 0, len(imageMap))
	for k := range imageMap {
		imageSlice = append(imageSlice, k)
	}
	return imageSlice
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
