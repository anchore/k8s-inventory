package pkg

import (
	"testing"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"

	"github.com/stretchr/testify/assert"
)

var (
	TestNamespace1 = inventory.Namespace{
		Name: "ns1",
	}
	TestNamespace2 = inventory.Namespace{
		Name: "ns2",
	}
	TestNamespace3 = inventory.Namespace{
		Name: "ns3",
	}
	TestNamespace4 = inventory.Namespace{
		Name: "ns4",
	}
	TestNamespaces = []inventory.Namespace{
		TestNamespace1,
		TestNamespace2,
		TestNamespace3,
		TestNamespace4,
	}
)

func TestGetAccountRoutedNamespaces(t *testing.T) {
	type args struct {
		defaultAccount string
		namespaces     []inventory.Namespace
		accountRoutes  config.AccountRoutes
	}
	tests := []struct {
		name string
		args args
		want map[string][]inventory.Namespace
	}{
		{
			name: "no account routes all to default",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes:  config.AccountRoutes{},
			},
			want: map[string][]inventory.Namespace{
				"admin": TestNamespaces,
			},
		},
		{
			name: "namespaces to individual accounts explicit",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes: config.AccountRoutes{
					"account1": config.AccountRouteDetails{
						Namespaces: []string{"ns1"},
					},
					"account2": config.AccountRouteDetails{
						Namespaces: []string{"ns2"},
					},
					"account3": config.AccountRouteDetails{
						Namespaces: []string{"ns3"},
					},
					"account4": config.AccountRouteDetails{
						Namespaces: []string{"ns4"},
					},
				},
			},
			want: map[string][]inventory.Namespace{
				"account1": {TestNamespace1},
				"account2": {TestNamespace2},
				"account3": {TestNamespace3},
				"account4": {TestNamespace4},
			},
		},
		{
			name: "namespaces to account regex",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes: config.AccountRoutes{
					"account1": config.AccountRouteDetails{
						Namespaces: []string{"ns.*"},
					},
				},
			},
			want: map[string][]inventory.Namespace{
				"account1": TestNamespaces,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAccountRoutedNamespaces(tt.args.defaultAccount, tt.args.namespaces, tt.args.accountRoutes)
			assert.Equal(t, tt.want, got)
		})
	}
}
