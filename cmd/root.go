package cmd

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/anchore/kai/kai/mode"

	"github.com/anchore/kai/kai"
	"github.com/anchore/kai/kai/presenter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

		switch appConfig.RunMode {
		case mode.PeriodicPolling:
			kai.PeriodicallyGetInventoryReport(appConfig)
		default:
			report, err := kai.GetInventoryReport(appConfig)
			if appConfig.Dev.ProfileCPU {
				pprof.StopCPUProfile()
			}
			if err != nil {
				log.Errorf("Failed to get Image Results: %+v", err)
				os.Exit(1)
			} else {
				err := kai.HandleReport(report, appConfig)
				if err != nil {
					log.Errorf("Failed to handle Image Results: %+v", err)
					os.Exit(1)
				}
			}
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
