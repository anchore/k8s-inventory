package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/anchore/k8s-inventory/pkg/healthreporter"
	"github.com/anchore/k8s-inventory/pkg/integration"
	"github.com/anchore/k8s-inventory/pkg/mode"
	"github.com/anchore/k8s-inventory/pkg/reporter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/anchore/k8s-inventory/pkg"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "anchore-k8s-inventory",
	Short: "anchore-k8s-inventory tells Anchore which images are in use in your Kubernetes Cluster",
	Long: `Anchore Kubernetes Inventory can poll
    Kubernetes Cluster API(s) to tell Anchore which Images are currently in-use`,
	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("anchore-k8s-inventory is starting up...")
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
				log.Error(err.Error())
				os.Exit(1)
			}
			os.Exit(1)
		}

		switch appConfig.RunMode {
		case mode.PeriodicPolling:
			neverDone := make(chan bool, 1)

			ch := integration.GetChannels()
			gatedReportInfo := healthreporter.GetGatedReportInfo()

			go healthreporter.PeriodicallySendHealthReport(appConfig, ch, gatedReportInfo)
			go pkg.PeriodicallyGetInventoryReport(appConfig, ch, gatedReportInfo)

			_, err := integration.PerformRegistration(appConfig, ch)
			if err != nil {
				os.Exit(1)
			}

			<-neverDone
		default:
			reports, err := pkg.GetInventoryReports(appConfig)
			if appConfig.Dev.ProfileCPU {
				pprof.StopCPUProfile()
			}
			if err != nil {
				log.Errorf("Failed to get Image Results: %+v", err)
				os.Exit(1)
			}
			anErrorOccurred := false
			reportInfo := healthreporter.InventoryReportInfo{}
			for account, reportsForAccount := range reports {
				for count, report := range reportsForAccount {
					log.Infof("Sending Inventory Report to Anchore Account %s, %d of %d", account, count+1, len(reportsForAccount))
					err = pkg.HandleReport(report, &reportInfo, appConfig, account)
					if errors.Is(err, reporter.ErrAnchoreAccountDoesNotExist) {
						// Retry with default account
						retryAccount := appConfig.AnchoreDetails.Account
						if appConfig.AccountRouteByNamespaceLabel.DefaultAccount != "" {
							retryAccount = appConfig.AccountRouteByNamespaceLabel.DefaultAccount
						}
						log.Warnf("Error sending to Anchore Account %s, sending to default account", account)
						err = pkg.HandleReport(report, &reportInfo, appConfig, retryAccount)
					}
					if err != nil {
						log.Errorf("Failed to handle Image Results: %+v", err)
						anErrorOccurred = true
					}
				}
			}
			if anErrorOccurred {
				os.Exit(1)
			}
		}
	},
}

func init() {
	opt := "kubeconfig"
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

	opt = "verbose-inventory-reports"
	rootCmd.Flags().BoolP(opt, "i", false, "If true, will print the full inventory report to stdout")
	if err := viper.BindPFlag(opt, rootCmd.Flags().Lookup(opt)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", opt, err)
		os.Exit(1)
	}
}
