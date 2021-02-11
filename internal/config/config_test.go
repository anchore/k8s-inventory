package config

import (
	"flag"
	"github.com/anchore/go-testutils"
	"github.com/spf13/viper"
	"testing"
)

var update = flag.Bool("update", false, "update the *.golden files for config string output")

func TestEmptyConfigString(t *testing.T) {
	config := &Application{}
	actual := config.String()

	if *update {
		t.Logf("Updating Golden file")
		testutils.UpdateGoldenFileContents(t, []byte(actual))
	}

	var expected = string(testutils.GetGoldenFileContents(t))
	if actual != expected {
		t.Errorf("Config string does not match expected\nactual: %s\nexpected: %s", actual, expected)
	}
}

func TestDefaultConfigString(t *testing.T) {
	config, err := LoadConfigFromFile(viper.GetViper(), &CliOnlyOptions{
		ConfigPath: "../../kai.yaml",
	})
	if err != nil {
		t.Errorf("failed to load application config: \n\t%+v\n", err)
	}
	actual := config.String()

	if *update {
		t.Logf("Updating Golden file")
		testutils.UpdateGoldenFileContents(t, []byte(actual))
	}

	var expected = string(testutils.GetGoldenFileContents(t))
	if actual != expected {
		t.Errorf("Config string does not match expected\nactual: %s\nexpected: %s", actual, expected)
	}
}

func TestSensitiveConfigString(t *testing.T) {
	config, err := LoadConfigFromFile(viper.GetViper(), &CliOnlyOptions{
		ConfigPath: "../../kai.yaml",
	})
	if err != nil {
		t.Errorf("failed to load application config: \n\t%+v\n", err)
	}
	config.AnchoreDetails.Password = "foo"
	config.KubeConfig.User.PrivateKey = "baz"
	config.KubeConfig.User.Token = "bar"
	actual := config.String()

	if *update {
		t.Logf("Updating Golden file")
		testutils.UpdateGoldenFileContents(t, []byte(actual))
	}

	var expected = string(testutils.GetGoldenFileContents(t))
	if actual != expected {
		t.Errorf("Config string does not match expected\nactual: %s\nexpected: %s", actual, expected)
	}
}
