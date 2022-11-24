/*
The Config package handles the application configuration. Configurations can come from a variety of places, and
are listed below in order of precedence:
  - Command Line
  - .kai.yaml
  - .kai/config.yaml
  - ~/.kai.yaml
  - <XDG_CONFIG_HOME>/kai/config.yaml
  - Environment Variables prefixed with KAI_
*/
package config

import (
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/anchore/kai/kai/mode"

	"github.com/adrg/xdg"
	"github.com/anchore/kai/internal"
	"github.com/anchore/kai/kai/presenter"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"path"
	"strings"
)

const redacted = "******"

// Configuration options that may only be specified on the command line
type CliOnlyOptions struct {
	ConfigPath string
	Verbosity  int
}

// All Application configurations
type Application struct {
	ConfigPath                      string
	PresenterOpt                    presenter.Option
	Output                          string  `mapstructure:"output"`
	Quiet                           bool    `mapstructure:"quiet"`
	Log                             Logging `mapstructure:"log"`
	CliOptions                      CliOnlyOptions
	Dev                             Development       `mapstructure:"dev"`
	KubeConfig                      KubeConf          `mapstructure:"kubeconfig"`
	Kubernetes                      KubernetesAPI     `mapstructure:"kubernetes"`
	Namespaces                      []string          `mapstructure:"namespaces"`
	KubernetesRequestTimeoutSeconds int64             `mapstructure:"kubernetes-request-timeout-seconds"`
	NamespaceSelectors              NamespaceSelector `mapstructure:"namespace-selectors"`
	MissingTagPolicy                MissingTagConf    `mapstructure:"missing-tag-policy"`
	RunMode                         mode.Mode
	Mode                            string      `mapstructure:"mode"`
	IgnoreNotRunning                bool        `mapstructure:"ignore-not-running"`
	PollingIntervalSeconds          int         `mapstructure:"polling-interval-seconds"`
	AnchoreDetails                  AnchoreInfo `mapstructure:"anchore"`
}

// MissingTagConf details the policy for handling missing tags when reporting images
type MissingTagConf struct {
	Policy string `mapstructure:"policy"`
	Tag    string `mapstructure:"tag,omitempty"`
}

// NamespaceSelector details the inclusion/exclusion rules for namespaces
type NamespaceSelector struct {
	Include []string `mapstructure:"include"`
	Exclude []string `mapstructure:"exclude"`
}

// KubernetesAPI details the configuration for interacting with the k8s api server
type KubernetesAPI struct {
	RequestTimeoutSeconds int64 `mapstructure:"request-timeout-seconds"`
	RequestBatchSize      int64 `mapstructure:"request-batch-size"`
	WorkerPoolSize        int   `mapstructure:"worker-pool-size"`
}

// Information for posting in-use image details to Anchore (or any URL for that matter)
type AnchoreInfo struct {
	URL      string     `mapstructure:"url"`
	User     string     `mapstructure:"user"`
	Password string     `mapstructure:"password"`
	Account  string     `mapstructure:"account"`
	HTTP     HTTPConfig `mapstructure:"http"`
}

// Configurations for the HTTP Client itself (net/http)
type HTTPConfig struct {
	Insecure       bool `mapstructure:"insecure"`
	TimeoutSeconds int  `mapstructure:"timeout-seconds"`
}

// Logging Configuration
type Logging struct {
	Structured   bool `mapstructure:"structured"`
	LevelOpt     logrus.Level
	Level        string `mapstructure:"level"`
	FileLocation string `mapstructure:"file"`
}

// Development Configuration (only profile-cpu at the moment)
type Development struct {
	ProfileCPU bool `mapstructure:"profile-cpu"`
}

// Return whether or not AnchoreDetails are specified
func (anchore *AnchoreInfo) IsValid() bool {
	return anchore.URL != "" &&
		anchore.User != "" &&
		anchore.Password != ""
}

func setNonCliDefaultValues(v *viper.Viper) {
	v.SetDefault("log.level", "")
	v.SetDefault("log.file", "")
	v.SetDefault("log.structured", false)
	v.SetDefault("dev.profile-cpu", false)
	v.SetDefault("anchore.account", "admin")
	v.SetDefault("kubeconfig.anchore.account", "admin")
	v.SetDefault("anchore.http.insecure", false)
	v.SetDefault("anchore.http.timeout-seconds", 10)
	v.SetDefault("kubernetes-request-timeout-seconds", -1)
	v.SetDefault("kubernetes.request-timeout-seconds", 60)
	v.SetDefault("kubernetes.request-batch-size", 100)
	v.SetDefault("kubernetes.worker-pool-size", 100)
	v.SetDefault("ignore-not-running", true)
	v.SetDefault("missing-tag-policy.policy", "digest")
	v.SetDefault("missing-tag-policy.tag", "UNKNOWN")
	v.SetDefault("namespaces", []string{})
	v.SetDefault("namespace-selectors.include", []string{})
	v.SetDefault("namespace-selectors.exclude", []string{})
}

// Load the Application Configuration from the Viper specifications
func LoadConfigFromFile(v *viper.Viper, cliOpts *CliOnlyOptions) (*Application, error) {
	// the user may not have a config, and this is OK, we can use the default config + default cobra cli values instead
	setNonCliDefaultValues(v)
	if cliOpts != nil {
		_ = readConfig(v, cliOpts.ConfigPath)
	} else {
		_ = readConfig(v, "")
	}

	config := &Application{
		CliOptions: *cliOpts,
	}
	err := v.Unmarshal(config)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}
	config.ConfigPath = v.ConfigFileUsed()

	err = config.Build()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// Build the configuration object (to be used as a singleton)
func (cfg *Application) Build() error {
	// set the presenter
	presenterOption := presenter.ParseOption(cfg.Output)
	if presenterOption == presenter.UnknownPresenter {
		return fmt.Errorf("bad --output value '%s'", cfg.Output)
	}
	cfg.PresenterOpt = presenterOption

	runMode := mode.ParseMode(cfg.Mode)
	cfg.RunMode = runMode

	if cfg.KubeConfig.User != (KubeConfUser{}) {
		cfg.KubeConfig.User.UserConfType = ParseUserConf(cfg.KubeConfig.User.UserConf)
	}

	if cfg.Quiet {
		// TODO: this is bad: quiet option trumps all other logging options
		// we should be able to quiet the console logging and leave file logging alone...
		// ... this will be an enhancement for later
		cfg.Log.LevelOpt = logrus.PanicLevel
	} else {
		if cfg.Log.Level != "" {
			if cfg.CliOptions.Verbosity > 0 {
				return fmt.Errorf("cannot explicitly set log level (cfg file or env var) and use -v flag together")
			}

			lvl, err := logrus.ParseLevel(strings.ToLower(cfg.Log.Level))
			if err != nil {
				return fmt.Errorf("bad log level configured (%q): %w", cfg.Log.Level, err)
			}
			// set the log level explicitly
			cfg.Log.LevelOpt = lvl
		} else {
			// set the log level implicitly
			switch v := cfg.CliOptions.Verbosity; {
			case v == 1:
				cfg.Log.LevelOpt = logrus.InfoLevel
			case v >= 2:
				cfg.Log.LevelOpt = logrus.DebugLevel
			default:
				cfg.Log.LevelOpt = logrus.ErrorLevel
			}
		}
	}

	// add new policies here if we decide to support more
	policies := []string{"digest", "insert", "drop"}
	validPolicy := false
	for _, p := range policies {
		if cfg.MissingTagPolicy.Policy == p {
			validPolicy = true
			break
		}
	}

	if !validPolicy {
		return fmt.Errorf("missing-tag-policy.policy must be one of %v", policies)
	}

	cfg.handleBackwardsCompatibility()

	return nil
}

func (cfg *Application) handleBackwardsCompatibility() {
	// BACKWARDS COMPATIBILITY - Translate namespaces into the new selector config
	// Only trigger if there is nothing in the include selector.
	if len(cfg.NamespaceSelectors.Include) == 0 && len(cfg.Namespaces) > 0 {
		for _, ns := range cfg.Namespaces {
			if ns == "all" {
				// set the include namespaces to an empty array if namespaces indicates collect "all"
				cfg.NamespaceSelectors.Include = []string{}
				break
			}
			// otherwise add the namespaces list to the include namespaces
			cfg.NamespaceSelectors.Include = append(cfg.NamespaceSelectors.Include, ns)
		}
	}

	// defer to the old config parameter if it is still present
	if cfg.KubernetesRequestTimeoutSeconds > 0 {
		cfg.Kubernetes.RequestTimeoutSeconds = cfg.KubernetesRequestTimeoutSeconds
	}
}

func readConfig(v *viper.Viper, configPath string) error {
	v.AutomaticEnv()
	v.SetEnvPrefix(internal.ApplicationName)
	// allow for nested options to be specified via environment variables
	// e.g. pod.context = APPNAME_POD_CONTEXT
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// use explicitly the given user config
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err == nil {
			return nil
		}
		// don't fall through to other options if this fails
		return fmt.Errorf("unable to read config: %v", configPath)
	}

	// start searching for valid configs in order...

	// 1. look for .<appname>.yaml (in the current directory)
	v.AddConfigPath(".")
	v.SetConfigName(internal.ApplicationName)
	if err := v.ReadInConfig(); err == nil {
		return nil
	}

	// 2. look for .<appname>/config.yaml (in the current directory)
	v.AddConfigPath("." + internal.ApplicationName)
	v.SetConfigName("config")
	if err := v.ReadInConfig(); err == nil {
		return nil
	}

	// 3. look for ~/.<appname>.yaml
	home, err := homedir.Dir()
	if err == nil {
		v.AddConfigPath(home)
		v.SetConfigName("." + internal.ApplicationName)
		if err := v.ReadInConfig(); err == nil {
			return nil
		}
	}

	// 4. look for <appname>/config.yaml in xdg locations (starting with xdg home config dir, then moving upwards)
	v.AddConfigPath(path.Join(xdg.ConfigHome, internal.ApplicationName))
	for _, dir := range xdg.ConfigDirs {
		v.AddConfigPath(path.Join(dir, internal.ApplicationName))
	}
	v.SetConfigName("config")
	if err := v.ReadInConfig(); err == nil {
		return nil
	}

	return fmt.Errorf("application config not found")
}

func (cfg Application) String() string {
	// redact sensitive information
	// Note: If the configuration grows to have more redacted fields it would be good to refactor this into something that
	// is more dynamic based on a property or list of "sensitive" fields
	if cfg.AnchoreDetails.Password != "" {
		cfg.AnchoreDetails.Password = redacted
	}

	if cfg.KubeConfig.User.PrivateKey != "" {
		cfg.KubeConfig.User.PrivateKey = redacted
	}

	if cfg.KubeConfig.User.Token != "" {
		cfg.KubeConfig.User.Token = redacted
	}

	// yaml is pretty human friendly (at least when compared to json)
	appCfgStr, err := yaml.Marshal(&cfg)

	if err != nil {
		return err.Error()
	}

	return string(appCfgStr)
}
