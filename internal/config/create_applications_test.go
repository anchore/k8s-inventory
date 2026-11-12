package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCreateApplicationsFrom(t *testing.T) {
	tests := []struct {
		name        string
		from        []string
		deployments bool
		want        []string
		wantErr     string
	}{
		{name: "unset", want: nil},
		{name: "all kinds", from: []string{"deployments", "statefulsets", "daemonsets"}, want: []string{"deployments", "statefulsets", "daemonsets"}},
		{name: "case, spaces and duplicates", from: []string{" StatefulSets", "statefulsets", "", "DaemonSets "}, want: []string{"statefulsets", "daemonsets"}},
		{name: "deprecated option adds deployments", from: []string{"statefulsets"}, deployments: true, want: []string{"statefulsets", "deployments"}},
		{name: "deprecated option already listed", from: []string{"deployments"}, deployments: true, want: []string{"deployments"}},
		{name: "unknown kind", from: []string{"deployments", "cronjobs"}, wantErr: `unknown workload kind "cronjobs"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Application{CreateApplicationsFrom: tt.from, CreateApplicationsFromDeployments: tt.deployments}
			err := cfg.normalizeCreateApplicationsFrom()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.CreateApplicationsFrom)
			assert.Equal(t, len(tt.want) > 0, cfg.CreatesApplications())
			for _, k := range tt.want {
				assert.True(t, cfg.CreatesApplicationsFrom(k))
			}
		})
	}
}

func loadWithConfig(t *testing.T, content string) (*Application, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return LoadConfigFromFile(viper.New(), &CliOnlyOptions{ConfigPath: path})
}

func TestCreateApplicationsFromSources(t *testing.T) {
	t.Run("config file list", func(t *testing.T) {
		cfg, err := loadWithConfig(t, "create-applications-from: [deployments, statefulsets]\n")
		require.NoError(t, err)
		assert.Equal(t, []string{"deployments", "statefulsets"}, cfg.CreateApplicationsFrom)
	})

	t.Run("comma separated environment variable", func(t *testing.T) {
		t.Setenv("ANCHORE_K8S_INVENTORY_CREATE_APPLICATIONS_FROM", "statefulsets,daemonsets")
		cfg, err := loadWithConfig(t, "{}\n")
		require.NoError(t, err)
		assert.Equal(t, []string{"statefulsets", "daemonsets"}, cfg.CreateApplicationsFrom)
	})

	t.Run("deprecated option from the preview", func(t *testing.T) {
		cfg, err := loadWithConfig(t, "create-applications-from-deployments: true\n")
		require.NoError(t, err)
		assert.Equal(t, []string{"deployments"}, cfg.CreateApplicationsFrom)
	})

	t.Run("invalid kind fails to load", func(t *testing.T) {
		_, err := loadWithConfig(t, "create-applications-from: [pods]\n")
		require.ErrorContains(t, err, "create-applications-from")
	})
}
