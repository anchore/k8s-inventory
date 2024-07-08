package cmd

import (
	"errors"
	"fmt"
	"github.com/anchore/k8s-inventory/pkg/healthreporter"
	"github.com/anchore/k8s-inventory/pkg/integration"
	"os"
	"runtime/pprof"

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
				log.Errorf(err.Error())
				os.Exit(1)
			}
			os.Exit(1)
		}

		instance, err := callHome()
		if err != nil {
			os.Exit(1)
		}

		switch appConfig.RunMode {
		case mode.PeriodicPolling:
			neverDone := make(chan bool, 1)

			gatedReportInfo := healthreporter.GatedReportInfo{
				AccountInventoryReports: make(healthreporter.AccountK8SInventoryReports, 0),
			}

			go pkg.PeriodicallyGetInventoryReport(appConfig, &gatedReportInfo)
			go healthreporter.PeriodicallySendHealthReportsGated(appConfig, &instance, &gatedReportInfo)

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

func callHome() (integration.Integration, error) {
	// TODO: Modify deployment.yaml in helm chart so that k8s-inventory-... pod can obtain namespace from env variable
	// envFrom:
	//   {{- if not .Values.injectSecretsViaEnv }}
	//       - secretRef:
	//           name: {{ default (include "k8sInventory.fullname" .) .Values.existingSecretName }}
	//   {{- end }}
	// env:
	//   - name: POD_NAMESPACE
	//           valueFrom:
	//             fieldRef:
	//               fieldPath: metadata.namespace
	namespace := os.Getenv("POD_NAMESPACE")
	name := os.Getenv("HOSTNAME")
	instance, err := pkg.GetIntegrationInfo(appConfig, namespace, name)
	if err != nil {
		log.Errorf("Failed to get Integration Info: %+v", err)
		return integration.Integration{}, err
	}
	// Register this agent with enterprise
	err = integration.Register(&instance, appConfig.AnchoreDetails)
	if err != nil {
		log.Errorf("Unable to register agent: %v", err)
		return integration.Integration{}, err
	}
	return instance, nil
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
