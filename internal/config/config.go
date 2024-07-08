/*
The Config package handles the application configuration. Configurations can come from a variety of places, and
are listed below in order of precedence:
  - Command Line
  - .anchore-k8s-inventory.yaml
  - .anchore-k8s-inventory/config.yaml
  - ~/.anchore-k8s-inventory.yaml
  - <XDG_CONFIG_HOME>/anchore-k8s-inventory/config.yaml
  - Environment Variables prefixed with ANCHORE_K8S_INVENTORY_
*/package config

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/anchore/k8s-inventory/pkg/mode"
	"gopkg.in/yaml.v2"

	"github.com/adrg/xdg"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/anchore/k8s-inventory/internal"
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
	Quiet                           bool    `mapstructure:"quiet" json:"quiet,omitempty" yaml:"quiet"`
	Log                             Logging `mapstructure:"log" json:"log,omitempty" yaml:"log"`
	CliOptions                      CliOnlyOptions
	Dev                             Development                  `mapstructure:"dev" json:"dev,omitempty" yaml:"dev"`
	KubeConfig                      KubeConf                     `mapstructure:"kubeconfig" json:"kubeconfig,omitempty" yaml:"kubeconfig"`
	Kubernetes                      KubernetesAPI                `mapstructure:"kubernetes" json:"kubernetes,omitempty" yaml:"kubernetes"`
	Namespaces                      []string                     `mapstructure:"namespaces" json:"namespaces,omitempty" yaml:"namespaces"`
	KubernetesRequestTimeoutSeconds int64                        `mapstructure:"kubernetes-request-timeout-seconds" json:"kubernetes-request-timeout-seconds,omitempty" yaml:"kubernetes-request-timeout-seconds"`
	NamespaceSelectors              NamespaceSelector            `mapstructure:"namespace-selectors" json:"namespace-selectors,omitempty" yaml:"namespace-selectors"`
	AccountRoutes                   AccountRoutes                `mapstructure:"account-routes" json:"account-routes,omitempty" yaml:"account-routes"`
	AccountRouteByNamespaceLabel    AccountRouteByNamespaceLabel `mapstructure:"account-route-by-namespace-label" json:"account-route-by-namespace-label,omitempty" yaml:"account-route-by-namespace-label"`
	MissingRegistryOverride         string                       `mapstructure:"missing-registry-override" json:"missing-registry-override,omitempty" yaml:"missing-registry-override"`
	MissingTagPolicy                MissingTagConf               `mapstructure:"missing-tag-policy" json:"missing-tag-policy,omitempty" yaml:"missing-tag-policy"`
	RunMode                         mode.Mode
	Mode                            string                `mapstructure:"mode" json:"mode,omitempty" yaml:"mode"`
	IgnoreNotRunning                bool                  `mapstructure:"ignore-not-running" json:"ignore-not-running,omitempty" yaml:"ignore-not-running"`
	PollingIntervalSeconds          int                   `mapstructure:"polling-interval-seconds" json:"polling-interval-seconds,omitempty" yaml:"polling-interval-seconds"`
	InventoryReportLimits           InventoryReportLimits `mapstructure:"inventory-report-limits" json:"inventory-report-limits,omitempty" yaml:"inventory-report-limits"`
	MetadataCollection              MetadataCollection    `mapstructure:"metadata-collection" json:"metadata-collection,omitempty" yaml:"metadata-collection"`
	AnchoreDetails                  AnchoreInfo           `mapstructure:"anchore" json:"anchore,omitempty" yaml:"anchore"`
	VerboseInventoryReports         bool                  `mapstructure:"verbose-inventory-reports" json:"verbose-inventory-reports,omitempty" yaml:"verbose-inventory-reports"`
}

// MissingTagConf details the policy for handling missing tags when reporting images
type MissingTagConf struct {
	Policy string `mapstructure:"policy" json:"policy,omitempty" yaml:"policy"`
	Tag    string `mapstructure:"tag,omitempty" json:"tag,omitempty" yaml:"tag"`
}

// NamespaceSelector details the inclusion/exclusion rules for namespaces
type NamespaceSelector struct {
	Include     []string `mapstructure:"include" json:"include,omitempty" yaml:"include"`
	Exclude     []string `mapstructure:"exclude" json:"exclude,omitempty" yaml:"exclude"`
	IgnoreEmpty bool     `mapstructure:"ignore-empty" json:"ignore-empty,omitempty" yaml:"ignore-empty"`
}

type AccountRoutes map[string]AccountRouteDetails

type AccountRouteDetails struct {
	User       string   `mapstructure:"user" json:"user,omitempty" yaml:"user"`
	Password   string   `mapstructure:"password" json:"password,omitempty" yaml:"password"`
	Namespaces []string `mapstructure:"namespaces" json:"namespaces,omitempty" yaml:"namespaces"`
}

type AccountRouteByNamespaceLabel struct {
	LabelKey           string `mapstructure:"key" json:"key,omitempty" yaml:"key"`
	DefaultAccount     string `mapstructure:"default-account" json:"default-account,omitempty" yaml:"default-account"`
	IgnoreMissingLabel bool   `mapstructure:"ignore-missing-label" json:"ignore-missing-label,omitempty" yaml:"ignore-missing-label"`
}

// KubernetesAPI details the configuration for interacting with the k8s api server
type KubernetesAPI struct {
	RequestTimeoutSeconds int64 `mapstructure:"request-timeout-seconds" json:"request-timeout-second,omitempty" yaml:"request-timeout-seconds"`
	RequestBatchSize      int64 `mapstructure:"request-batch-size" json:"request-batch-size,omitempty" yaml:"request-batch-size"`
	WorkerPoolSize        int   `mapstructure:"worker-pool-size" json:"worker-pool-size,omitempty" yaml:"worker-pool-size"`
}

// Details upper limits for the inventory report contents before splitting into batches
type InventoryReportLimits struct {
	Namespaces int `mapstructure:"namespaces" json:"namespaces,omitempty" yaml:"namespaces"`
}

type ResourceMetadata struct {
	Annotations []string `mapstructure:"include-annotations" json:"include-annotations,omitempty" yaml:"include-annotations"`
	Labels      []string `mapstructure:"include-labels" json:"include-labels,omitempty" yaml:"include-labels"`
	Disable     bool     `mapstructure:"disable" json:"disable,omitempty" yaml:"disable"`
}

type MetadataCollection struct {
	Nodes     ResourceMetadata `mapstructure:"nodes" json:"nodes,omitempty" yaml:"nodes"`
	Namespace ResourceMetadata `mapstructure:"namespaces" json:"namespace,omitempty" yaml:"namespaces"`
	Pods      ResourceMetadata `mapstructure:"pods" json:"pods,omitempty" yaml:"pods"`
}

// Information for posting in-use image details to Anchore (or any URL for that matter)
type AnchoreInfo struct {
	URL      string     `mapstructure:"url" json:"url,omitempty" yaml:"url"`
	User     string     `mapstructure:"user" json:"user,omitempty" yaml:"user"`
	Password string     `mapstructure:"password" json:"password,omitempty" yaml:"password"`
	Account  string     `mapstructure:"account" json:"account,omitempty" yaml:"account"`
	HTTP     HTTPConfig `mapstructure:"http" json:"http,omitempty" yaml:"http"`
}

// Configurations for the HTTP Client itself (net/http)
type HTTPConfig struct {
	Insecure       bool `mapstructure:"insecure" json:"insecure,omitempty" yaml:"insecure"`
	TimeoutSeconds int  `mapstructure:"timeout-seconds" json:"timeout-seconds,omitempty" yaml:"timeout-seconds"`
}

// Logging Configuration
type Logging struct {
	Structured   bool `mapstructure:"structured" json:"structured,omitempty" yaml:"structured"`
	LevelOpt     logrus.Level
	Level        string `mapstructure:"level" json:"level,omitempty" yaml:"level"`
	FileLocation string `mapstructure:"file" json:"file,omitempty" yaml:"file"`
}

// Development Configuration (only profile-cpu at the moment)
type Development struct {
	ProfileCPU bool `mapstructure:"profile-cpu" json:"profile-cpu,omitempty" yaml:"profile-cpu"`
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
	v.SetDefault("missing-registry-override", "")
	v.SetDefault("missing-tag-policy.policy", "digest")
	v.SetDefault("missing-tag-policy.tag", "UNKNOWN")
	v.SetDefault("account-routes", AccountRoutes{})
	v.SetDefault("account-route-by-namespace-label", AccountRouteByNamespaceLabel{})
	v.SetDefault("namespaces", []string{})
	v.SetDefault("namespace-selectors.include", []string{})
	v.SetDefault("namespace-selectors.exclude", []string{})
	v.SetDefault("namespace-selectors.ignore-empty", false)
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
				cfg.Log.LevelOpt = logrus.InfoLevel
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
	// yaml is pretty human friendly (at least when compared to json)
	appCfgStr, err := yaml.Marshal(&cfg)
	if err != nil {
		return err.Error()
	}

	return string(appCfgStr)
}

func (anchore AnchoreInfo) MarshalJSON() ([]byte, error) {
	type anchoreInfoAlias AnchoreInfo // prevent recursion

	aIA := anchoreInfoAlias(anchore)
	if aIA.Password != "" {
		aIA.Password = redacted
	}
	return json.Marshal(aIA)
}

func (anchore AnchoreInfo) MarshalYAML() (interface{}, error) {
	if anchore.Password != "" {
		anchore.Password = redacted
	}
	return anchore, nil
}

func (aRD AccountRouteDetails) MarshalJSON() ([]byte, error) {
	type AccountRouteDetailsAlias AccountRouteDetails // prevent recursion

	aRDA := AccountRouteDetailsAlias(aRD)
	if aRDA.Password != "" {
		aRDA.Password = redacted
	}
	return json.Marshal(aRDA)
}

func (aRD AccountRouteDetails) MarshalYAML() (interface{}, error) {
	if aRD.Password != "" {
		aRD.Password = redacted
	}
	return aRD, nil
}
