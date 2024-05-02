// Wraps some of the initialization details for the k8s clientset
package client

import (
	"testing"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestGetClientSet(t *testing.T) {
	type args struct {
		kubeConfig *rest.Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				kubeConfig: &rest.Config{},
			},
			wantErr: false,
		},
		{
			name: "sad path",
			args: args{
				kubeConfig: &rest.Config{
					AuthProvider: &clientcmdapi.AuthProviderConfig{},
					ExecProvider: &clientcmdapi.ExecConfig{},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetClientSet(tt.args.kubeConfig)
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetKubeConfig(t *testing.T) {
	type args struct {
		appConfig *config.Application
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "use default",
			args: args{
				appConfig: &config.Application{},
			},
			wantErr: false,
		},
		{
			name: "use in-cluster",
			args: args{
				appConfig: &config.Application{
					KubeConfig: config.KubeConf{
						Path: "use-in-cluster",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetKubeConfig(tt.args.appConfig)
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}
