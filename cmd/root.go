package cmd

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/anchore/kai/internal/config"

	"github.com/anchore/kai/kai/client"
	"k8s.io/client-go/rest"

	"github.com/anchore/kai/kai/mode"

	"github.com/anchore/kai/internal"
	"github.com/anchore/kai/internal/bus"
	"github.com/anchore/kai/internal/ui"
	"github.com/anchore/kai/internal/version"
	"github.com/anchore/kai/kai"
	"github.com/anchore/kai/kai/event"
	"github.com/anchore/kai/kai/presenter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wagoodman/go-partybus"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kai",
	Short: "KAI tells Anchore which images are in use in your Kubernetes Cluster",
	Long: `KAI (Kubernetes Automated Inventory) can poll 
    Kubernetes Cluster API(s) to tell Anchore which Images are currently in-use`,
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
	rootCmd.Flags().StringP(opt, "k", "", "(optional) absolute path to the kubeconfig file")
	if err := viper.BindPFlag(opt+".path", rootCmd.Flags().Lookup(opt)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", opt, err)
		os.Exit(1)
	}

	opt = "namespaces"
	rootCmd.Flags().StringSliceP(opt, "n", []string{"all"}, "(optional) namespaces to search")
	err := rootCmd.RegisterFlagCompletionFunc(opt, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		namespaces, err := kai.ListNamespaces(appConfig)
		if err != nil {
			return []string{"completion failed"}, cobra.ShellCompDirectiveError
		}
		return append(namespaces, "all"), cobra.ShellCompDirectiveDefault
	})
	if err != nil {
		fmt.Printf("unable to register flag completion script for \"namespace\": %+v", err)
	}
	if err := viper.BindPFlag(opt, rootCmd.Flags().Lookup(opt)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", opt, err)
		os.Exit(1)
	}

	opt = "mode"
	rootCmd.Flags().StringP(opt, "m", mode.AdHoc.String(), fmt.Sprintf("execution mode, options=%v", mode.Modes))
	if err := viper.BindPFlag(opt, rootCmd.Flags().Lookup(opt)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", opt, err)
		os.Exit(1)
	}

	opt = "polling-interval-seconds"
	rootCmd.Flags().StringP(opt, "p", "300", "If mode is 'periodic', this specifies the interval")
	if err := viper.BindPFlag(opt, rootCmd.Flags().Lookup(opt)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", opt, err)
		os.Exit(1)
	}
}

func getImageResults() <-chan error {
	errs := make(chan error)
	go func() {
		defer close(errs)

		checkForAppUpdateIfEnabled()

		// In this case, there may be multiple Clusters to pull from
		if appConfig.KubeConfig.IsKubeConfigFromAnchore() {
			anchoreClusterConfigs := pollAnchoreForClusterConfigs(errs)
			if anchoreClusterConfigs == nil {
				log.Fatal("Failed to get Cluster Configs from Anchore")
				return
			}
			for _, clusterConfig := range anchoreClusterConfigs {
				kubeConfig, err := clusterConfig.ToKubeConfig()
				if err != nil {
					errs <- err
					return
				}
				go getImageResultsAccordingToRunMode(errs, kubeConfig, clusterConfig.ClusterName, clusterConfig.Namespaces)
			}
			return
		}

		kubeConfig, err := client.GetKubeConfig(appConfig)
		if err != nil {
			errs <- err
			return
		}
		getImageResultsAccordingToRunMode(errs, kubeConfig, appConfig.KubeConfig.Cluster, appConfig.Namespaces)
	}()
	return errs
}

func pollAnchoreForClusterConfigs(errs chan error) []config.AnchoreClusterConfig {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		anchoreClusterConfigs, err := appConfig.KubeConfig.GetClusterConfigsFromAnchore()
		if err != nil {
			errs <- err
			break
		} else {
			if len(anchoreClusterConfigs) == 0 {
				log.Warn("no cluster configurations found from Anchore")
			} else {
				return anchoreClusterConfigs
			}
		}
	}
	return nil
}

func getImageResultsAccordingToRunMode(errs chan error, kubeConfig *rest.Config, clusterName string, namespaces []string) {
	switch appConfig.RunMode {
	case mode.PeriodicPolling:
		kai.PeriodicallyGetImageResults(errs, appConfig, kubeConfig, clusterName, namespaces)
	default:
		imagesResult := kai.GetImageResults(errs, kubeConfig, clusterName, namespaces)

		bus.Publish(partybus.Event{
			Type:   event.ImageResultsRetrieved,
			Source: imagesResult,
			Value:  presenter.GetPresenter(appConfig.PresenterOpt, imagesResult),
		})
	}
}

func runDefaultCmd() error {
	errs := getImageResults()
	return ui.LoggerUI(errs, eventSubscription, appConfig)
}

func checkForAppUpdateIfEnabled() {
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
}
