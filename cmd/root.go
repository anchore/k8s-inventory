package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"

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
	home := homeDir()
	rootCmd.Flags().StringP(opt, "k", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	if err := viper.BindPFlag(opt, rootCmd.Flags().Lookup(opt)); err != nil {
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

		switch appConfig.RunMode {
		case mode.PeriodicPolling:
			kai.PeriodicallyGetImageResults(errs, appConfig)
		default:
			imagesResult := kai.GetImageResults(errs, appConfig)

			bus.Publish(partybus.Event{
				Type:   event.ImageResultsRetrieved,
				Source: imagesResult,
				Value:  presenter.GetPresenter(appConfig.PresenterOpt, imagesResult),
			})
		}
	}()
	return errs
}

func runDefaultCmd() error {
	errs := getImageResults()
	return ui.LoggerUI(errs, eventSubscription, appConfig)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
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
