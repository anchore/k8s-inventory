package config

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/anchore/kai/internal/log"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const ClusterConfigsAPIPath = "v1/enterprise/images/inventory/kubernetes/clusters"

// Defines how the Kubernetes Client should be configured. Note: Doesn't seem to work well with Env vars
type KubeConf struct {
	Path           string       `mapstructure:"path"`
	AnchoreDetails AnchoreInfo  `mapstructure:"anchore"`
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

func (kubeConf *KubeConf) IsKubeConfigFromAnchore() bool {
	return kubeConf.AnchoreDetails.IsValid()
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

type AnchoreClusterConfig struct {
	ID                 string   `json:"id"`
	Account            string   `json:"account"`
	ClusterName        string   `json:"cluster_name"`
	InventoryType      string   `json:"inventory_type"`
	CredentialType     string   `json:"credential_type"`
	Namespaces         []string `json:"namespaces"`
	ClusterServer      string   `json:"cluster_server"`
	ClusterCertificate string   `json:"cluster_certificate"`
	ClientCertificate  string   `json:"client_certificate"`
	Credential         string   `json:"credential"`
}

//nolint:gosec
func (kubeConf *KubeConf) downloadKubeConfigsFromAnchore() ([]byte, error) {
	anchoreDetails := kubeConf.AnchoreDetails
	log.Debug("Retrieving Cluster configurations from Anchore")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: anchoreDetails.HTTP.Insecure},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(anchoreDetails.HTTP.TimeoutSeconds) * time.Second,
	}

	anchoreURL, err := url.Parse(anchoreDetails.URL)
	if err != nil {
		return nil, err
	}

	anchoreURL.Path += ClusterConfigsAPIPath

	req, err := http.NewRequest("GET", anchoreURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request to get cluster configs from anchore: %w", err)
	}

	req.SetBasicAuth(anchoreDetails.User, anchoreDetails.Password)
	req.Header.Set("x-anchore-account", anchoreDetails.Account)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster configs from anchore: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to retrieve cluster configs from anchore: %+v", resp)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster config body: %w", err)
	}
	return body, nil
}

func (kubeConf *KubeConf) GetClusterConfigsFromAnchore() ([]AnchoreClusterConfig, error) {
	body, err := kubeConf.downloadKubeConfigsFromAnchore()
	if err != nil {
		return nil, err
	}

	configs := make([]AnchoreClusterConfig, 0)
	err = json.Unmarshal(body, &configs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster config body as json: %w", err)
	}
	return configs, nil
}

func (clusterConfig *AnchoreClusterConfig) ToKubeConfig() (*rest.Config, error) {
	clusters := make(map[string]*api.Cluster)
	decodedCert, err := base64.StdEncoding.DecodeString(clusterConfig.ClusterCertificate)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode cluster cert: %w", err)
	}
	clusters[clusterConfig.ClusterName] = &api.Cluster{
		CertificateAuthorityData: decodedCert,
		Server:                   clusterConfig.ClusterServer,
	}

	authInfos := make(map[string]*api.AuthInfo)
	userConfType := ParseUserConf(clusterConfig.CredentialType)
	if userConfType == PrivateKey {
		decodedClientCert, err := base64.StdEncoding.DecodeString(clusterConfig.ClientCertificate)
		if err != nil {
			return nil, fmt.Errorf("failed to base64 decode client cert: %w", err)
		}

		decodedPrivateKey, err := base64.StdEncoding.DecodeString(clusterConfig.Credential)
		if err != nil {
			return nil, fmt.Errorf("failed to base64 decode private key: %w", err)
		}

		authInfos[clusterConfig.ClusterName] = &api.AuthInfo{
			ClientCertificateData: decodedClientCert,
			ClientKeyData:         decodedPrivateKey,
		}
	} else if userConfType == ServiceAccountToken {
		authInfos[clusterConfig.ClusterName] = &api.AuthInfo{
			Token: clusterConfig.Credential,
		}
	}

	contexts := make(map[string]*api.Context)
	contexts[clusterConfig.ClusterName] = &api.Context{
		Cluster:  clusterConfig.ClusterName,
		AuthInfo: clusterConfig.ClusterName,
	}

	apiConfig := api.Config{
		Kind:           "config",
		APIVersion:     "v1",
		Clusters:       clusters,
		AuthInfos:      authInfos,
		Contexts:       contexts,
		CurrentContext: clusterConfig.ClusterName,
	}

	return clientcmd.NewDefaultClientConfig(apiConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
}
