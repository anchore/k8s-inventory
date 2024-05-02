package config

import (
	"flag"
	"testing"

	"github.com/anchore/go-testutils"
	"github.com/spf13/viper"
)

var update = flag.Bool("update", false, "update the *.golden files for config string output")

func TestEmptyConfigString(t *testing.T) {
	config := &Application{}
	actual := config.String()

	if *update {
		t.Logf("Updating Golden file")
		testutils.UpdateGoldenFileContents(t, []byte(actual))
	}

	expected := string(testutils.GetGoldenFileContents(t))
	if actual != expected {
		t.Errorf("Config string does not match expected\nactual: %s\nexpected: %s", actual, expected)
	}
}

func TestDefaultConfigString(t *testing.T) {
	config, err := LoadConfigFromFile(viper.GetViper(), &CliOnlyOptions{
		ConfigPath: "../../anchore-k8s-inventory.yaml",
	})
	if err != nil {
		t.Errorf("failed to load application config: \n\t%+v\n", err)
	}
	actual := config.String()

	if *update {
		t.Logf("Updating Golden file")
		testutils.UpdateGoldenFileContents(t, []byte(actual))
	}

	expected := string(testutils.GetGoldenFileContents(t))
	if actual != expected {
		t.Errorf("Config string does not match expected\nactual: %s\nexpected: %s", actual, expected)
	}
}

func TestSensitiveConfigString(t *testing.T) {
	config, err := LoadConfigFromFile(viper.GetViper(), &CliOnlyOptions{
		ConfigPath: "../../anchore-k8s-inventory.yaml",
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

	expected := string(testutils.GetGoldenFileContents(t))
	if actual != expected {
		t.Errorf("Config string does not match expected\nactual: %s\nexpected: %s", actual, expected)
	}
}

func TestAnchoreInfo_IsValid(t *testing.T) {
	type fields struct {
		URL      string
		User     string
		Password string
		Account  string
		HTTP     HTTPConfig
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "valid",
			fields: fields{
				URL:      "http://anchore.example.com",
				User:     "admin",
				Password: "foobar",
				Account:  "admin",
				HTTP:     HTTPConfig{},
			},
			want: true,
		},
		{
			name: "invalid",
			fields: fields{
				URL:      "http://anchore.example.com",
				User:     "",
				Password: "foobar",
				Account:  "admin",
				HTTP:     HTTPConfig{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchore := &AnchoreInfo{
				URL:      tt.fields.URL,
				User:     tt.fields.User,
				Password: tt.fields.Password,
				Account:  tt.fields.Account,
				HTTP:     tt.fields.HTTP,
			}
			if got := anchore.IsValid(); got != tt.want {
				t.Errorf("AnchoreInfo.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
