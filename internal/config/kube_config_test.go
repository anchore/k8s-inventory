package config

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeConf_IsKubeConfigFromFile(t *testing.T) {
	tests := []struct {
		name string
		conf KubeConf
		want bool
	}{
		{
			name: "path set",
			conf: KubeConf{Path: "/home/user/.kube/config"},
			want: true,
		},
		{
			name: "path empty",
			conf: KubeConf{},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.conf.IsKubeConfigFromFile())
		})
	}
}

func TestKubeConf_IsNonFileKubeConfigValid(t *testing.T) {
	validPrivateKeyUser := KubeConfUser{
		UserConfType: PrivateKey,
		ClientCert:   "cert-data",
		PrivateKey:   "key-data",
	}
	validTokenUser := KubeConfUser{
		UserConfType: ServiceAccountToken,
		Token:        "my-token",
	}

	tests := []struct {
		name string
		conf KubeConf
		want bool
	}{
		{
			name: "all fields set with private key user",
			conf: KubeConf{
				Cluster:     "my-cluster",
				ClusterCert: "cert",
				Server:      "https://k8s.example.com",
				User:        validPrivateKeyUser,
			},
			want: true,
		},
		{
			name: "all fields set with token user",
			conf: KubeConf{
				Cluster:     "my-cluster",
				ClusterCert: "cert",
				Server:      "https://k8s.example.com",
				User:        validTokenUser,
			},
			want: true,
		},
		{
			name: "missing cluster",
			conf: KubeConf{
				ClusterCert: "cert",
				Server:      "https://k8s.example.com",
				User:        validPrivateKeyUser,
			},
			want: false,
		},
		{
			name: "missing cluster cert",
			conf: KubeConf{
				Cluster: "my-cluster",
				Server:  "https://k8s.example.com",
				User:    validPrivateKeyUser,
			},
			want: false,
		},
		{
			name: "missing server",
			conf: KubeConf{
				Cluster:     "my-cluster",
				ClusterCert: "cert",
				User:        validPrivateKeyUser,
			},
			want: false,
		},
		{
			name: "invalid user (private key missing cert)",
			conf: KubeConf{
				Cluster:     "my-cluster",
				ClusterCert: "cert",
				Server:      "https://k8s.example.com",
				User: KubeConfUser{
					UserConfType: PrivateKey,
					PrivateKey:   "key-data",
				},
			},
			want: false,
		},
		{
			name: "invalid user (token missing)",
			conf: KubeConf{
				Cluster:     "my-cluster",
				ClusterCert: "cert",
				Server:      "https://k8s.example.com",
				User: KubeConfUser{
					UserConfType: ServiceAccountToken,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.conf.IsNonFileKubeConfigValid())
		})
	}
}

func TestKubeConfUser_isValid(t *testing.T) {
	tests := []struct {
		name string
		user KubeConfUser
		want bool
	}{
		{
			name: "private key with cert and key",
			user: KubeConfUser{
				UserConfType: PrivateKey,
				ClientCert:   "cert",
				PrivateKey:   "key",
			},
			want: true,
		},
		{
			name: "private key missing client cert",
			user: KubeConfUser{
				UserConfType: PrivateKey,
				PrivateKey:   "key",
			},
			want: false,
		},
		{
			name: "private key missing private key",
			user: KubeConfUser{
				UserConfType: PrivateKey,
				ClientCert:   "cert",
			},
			want: false,
		},
		{
			name: "service account token with token",
			user: KubeConfUser{
				UserConfType: ServiceAccountToken,
				Token:        "my-token",
			},
			want: true,
		},
		{
			name: "service account token without token",
			user: KubeConfUser{
				UserConfType: ServiceAccountToken,
			},
			want: false,
		},
		{
			name: "default user type is always valid",
			user: KubeConfUser{
				UserConfType: UserConf(99),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.user.isValid())
		})
	}
}

func TestKubeConf_GetKubeConfigFromConf_ServiceAccountToken(t *testing.T) {
	cert := "test-ca-cert-data"
	encodedCert := base64.StdEncoding.EncodeToString([]byte(cert))

	kubeConf := &KubeConf{
		Cluster:     "test-cluster",
		ClusterCert: encodedCert,
		Server:      "https://k8s.example.com:6443",
		User: KubeConfUser{
			UserConfType: ServiceAccountToken,
			Token:        "my-sa-token",
		},
	}

	restConfig, err := kubeConf.GetKubeConfigFromConf()
	require.NoError(t, err)
	assert.Equal(t, "https://k8s.example.com:6443", restConfig.Host)
	assert.Equal(t, "my-sa-token", restConfig.BearerToken)
}

func TestKubeConf_GetKubeConfigFromConf_PrivateKey(t *testing.T) {
	cert := "test-ca-cert-data"
	clientCert := "test-client-cert"
	privateKey := "test-private-key"

	kubeConf := &KubeConf{
		Cluster:     "test-cluster",
		ClusterCert: base64.StdEncoding.EncodeToString([]byte(cert)),
		Server:      "https://k8s.example.com:6443",
		User: KubeConfUser{
			UserConfType: PrivateKey,
			ClientCert:   base64.StdEncoding.EncodeToString([]byte(clientCert)),
			PrivateKey:   base64.StdEncoding.EncodeToString([]byte(privateKey)),
		},
	}

	restConfig, err := kubeConf.GetKubeConfigFromConf()
	require.NoError(t, err)
	assert.Equal(t, "https://k8s.example.com:6443", restConfig.Host)
	assert.Equal(t, []byte(clientCert), restConfig.CertData)
	assert.Equal(t, []byte(privateKey), restConfig.KeyData)
}

func TestKubeConf_GetKubeConfigFromConf_InvalidBase64Cert(t *testing.T) {
	kubeConf := &KubeConf{
		Cluster:     "test-cluster",
		ClusterCert: "not-valid-base64!!!",
		Server:      "https://k8s.example.com:6443",
		User: KubeConfUser{
			UserConfType: ServiceAccountToken,
			Token:        "token",
		},
	}

	_, err := kubeConf.GetKubeConfigFromConf()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to base64 decode cluster cert")
}

func TestKubeConf_getAuthInfosFromConf_InvalidBase64ClientCert(t *testing.T) {
	kubeConf := &KubeConf{
		Cluster: "test-cluster",
		User: KubeConfUser{
			UserConfType: PrivateKey,
			ClientCert:   "not-valid-base64!!!",
			PrivateKey:   base64.StdEncoding.EncodeToString([]byte("key")),
		},
	}

	_, err := kubeConf.getAuthInfosFromConf()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to base64 decode client cert")
}

func TestKubeConf_getAuthInfosFromConf_InvalidBase64PrivateKey(t *testing.T) {
	kubeConf := &KubeConf{
		Cluster: "test-cluster",
		User: KubeConfUser{
			UserConfType: PrivateKey,
			ClientCert:   base64.StdEncoding.EncodeToString([]byte("cert")),
			PrivateKey:   "not-valid-base64!!!",
		},
	}

	_, err := kubeConf.getAuthInfosFromConf()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to base64 decode private key")
}

func TestKubeConfUser_MarshalJSON_RedactsSensitiveFields(t *testing.T) {
	user := KubeConfUser{
		UserConfType: PrivateKey,
		ClientCert:   "visible-cert",
		PrivateKey:   "secret-key",
		Token:        "secret-token",
	}

	data, err := json.Marshal(user)
	require.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "visible-cert", unmarshaled["client-cert"])
	assert.Equal(t, redacted, unmarshaled["private-key"])
	assert.Equal(t, redacted, unmarshaled["token"])
}

func TestKubeConfUser_MarshalYAML_RedactsSensitiveFields(t *testing.T) {
	user := KubeConfUser{
		UserConfType: PrivateKey,
		ClientCert:   "visible-cert",
		PrivateKey:   "secret-key",
		Token:        "secret-token",
	}

	result, err := user.MarshalYAML()
	require.NoError(t, err)

	redactedUser, ok := result.(KubeConfUser)
	require.True(t, ok)
	assert.Equal(t, "visible-cert", redactedUser.ClientCert)
	assert.Equal(t, redacted, redactedUser.PrivateKey)
	assert.Equal(t, redacted, redactedUser.Token)
}
