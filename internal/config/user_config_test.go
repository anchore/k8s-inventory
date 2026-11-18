package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseUserConf(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  UserConf
	}{
		{
			name:  "token string",
			input: "token",
			want:  ServiceAccountToken,
		},
		{
			name:  "token uppercase",
			input: "TOKEN",
			want:  ServiceAccountToken,
		},
		{
			name:  "token mixed case",
			input: "Token",
			want:  ServiceAccountToken,
		},
		{
			name:  "private_key string",
			input: "private_key",
			want:  PrivateKey,
		},
		{
			name:  "unknown defaults to PrivateKey",
			input: "unknown",
			want:  PrivateKey,
		},
		{
			name:  "empty defaults to PrivateKey",
			input: "",
			want:  PrivateKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseUserConf(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserConf_String(t *testing.T) {
	tests := []struct {
		name string
		conf UserConf
		want string
	}{
		{
			name: "PrivateKey",
			conf: PrivateKey,
			want: "private_key",
		},
		{
			name: "ServiceAccountToken",
			conf: ServiceAccountToken,
			want: "token",
		},
		{
			name: "negative value defaults to private_key",
			conf: UserConf(-1),
			want: "private_key",
		},
		{
			name: "out of range value defaults to private_key",
			conf: UserConf(99),
			want: "private_key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.conf.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
