package config

import (
	"encoding/base64"
	"fmt"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Defines how the Kubernetes Client should be configured. Note: Doesn't seem to work well with Env vars
type KubeConf struct {
	Path           string       `mapstructure:"path"`
	Cluster        string       `mapstructure:"cluster"`
	ClusterCert    string       `mapstructure:"cluster-cert"`
	Server         string       `mapstructure:"server"`
	User           KubeConfUser `mapstructure:"user"`
}

// If we are explicitly providing authentication information (not from a kubeconfig file), we need this info
type KubeConfUser struct {
	UserConfType UserConf
	UserConf     string `mapstructure:"type"`
	ClientCert   string `mapstructure:"client-cert"`
	PrivateKey   string `mapstructure:"private-key"`
	Token        string `mapstructure:"token"`
}

func (kubeConf *KubeConf) IsKubeConfigFromFile() bool {
	return kubeConf.Path != ""
}

func (kubeConf *KubeConf) IsNonFileKubeConfigValid() bool {
	return kubeConf.Cluster != "" && kubeConf.ClusterCert != "" && kubeConf.Server != "" && kubeConf.User.isValid()
}

func (user *KubeConfUser) isValid() bool {
	result := true
	if user.UserConfType == PrivateKey {
		result = user.ClientCert != "" && user.PrivateKey != ""
	} else if user.UserConfType == ServiceAccountToken {
		result = user.Token != ""
	}
	return result
}

func (kubeConf *KubeConf) GetKubeConfigFromConf() (*rest.Config, error) {
	clusters := make(map[string]*api.Cluster)
	decodedCert, err := base64.StdEncoding.DecodeString(kubeConf.ClusterCert)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode cluster cert: %w", err)
	}
	clusters[kubeConf.Cluster] = &api.Cluster{
		CertificateAuthorityData: decodedCert,
		Server:                   kubeConf.Server,
	}

	users, err := kubeConf.getAuthInfosFromConf()
	if err != nil {
		return nil, err
	}

	contexts := make(map[string]*api.Context)
	contexts[kubeConf.Cluster] = &api.Context{
		Cluster:  kubeConf.Cluster,
		AuthInfo: kubeConf.Cluster,
	}

	apiConfig := api.Config{
		Kind:           "config",
		APIVersion:     "v1",
		Clusters:       clusters,
		AuthInfos:      users,
		Contexts:       contexts,
		CurrentContext: kubeConf.Cluster,
	}

	return clientcmd.NewDefaultClientConfig(apiConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
}

func (kubeConf *KubeConf) getAuthInfosFromConf() (map[string]*api.AuthInfo, error) {
	authInfos := make(map[string]*api.AuthInfo)
	cluster := kubeConf.Cluster
	userConf := kubeConf.User
	userConfType := userConf.UserConfType
	if userConfType == PrivateKey {
		decodedClientCert, err := base64.StdEncoding.DecodeString(userConf.ClientCert)
		if err != nil {
			return nil, fmt.Errorf("failed to base64 decode client cert: %w", err)
		}

		decodedPrivateKey, err := base64.StdEncoding.DecodeString(userConf.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to base64 decode private key: %w", err)
		}

		authInfos[cluster] = &api.AuthInfo{
			ClientCertificateData: decodedClientCert,
			ClientKeyData:         decodedPrivateKey,
		}
	} else if userConfType == ServiceAccountToken {
		authInfos[cluster] = &api.AuthInfo{
			Token: kubeConf.User.Token,
		}
	}
	return authInfos, nil
}