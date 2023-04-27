package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/anchore/k8s-inventory/internal"
	"github.com/anchore/k8s-inventory/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show the version",
	Run:   printVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersion(_ *cobra.Command, _ []string) {
	versionInfo := version.FromBuild()
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	err := enc.Encode(&struct {
		version.Version
		Application string `json:"application"`
	}{
		Version:     versionInfo,
		Application: internal.ApplicationName,
	})
	if err != nil {
		fmt.Printf("failed to show version information: %+v\n", err)
		os.Exit(1)
	}
}
