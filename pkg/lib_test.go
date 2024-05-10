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
		Labels: map[string]string{
			"anchore.io/account": "account1",
		},
	}
	TestNamespace2 = inventory.Namespace{
		Name: "ns2",
		Labels: map[string]string{
			"anchore.io/account": "account2",
		},
	}
	TestNamespace3 = inventory.Namespace{
		Name: "ns3",
		Labels: map[string]string{
			"anchore.io/account": "account3",
		},
	}
	TestNamespace4 = inventory.Namespace{
		Name: "ns4",
		Labels: map[string]string{
			"anchore.io/account": "account4",
		},
	}
	TestNamespace5 = inventory.Namespace{
		Name:   "ns5-no-label",
		Labels: map[string]string{},
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
		defaultAccount        string
		namespaces            []inventory.Namespace
		accountRoutes         config.AccountRoutes
		namespaceLabelRouting config.AccountRouteByNamespaceLabel
	}
	tests := []struct {
		name string
		args args
		want map[string][]inventory.Namespace
	}{
		{
			name: "no account routes all to default",
			args: args{
				defaultAccount:        "admin",
				namespaces:            TestNamespaces,
				accountRoutes:         config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{},
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
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{},
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
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{},
			},
			want: map[string][]inventory.Namespace{
				"account1": TestNamespaces,
			},
		},
		{
			name: "namespaces to accounts that match a label only",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "default",
					IgnoreMissingLabel: false,
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
			name: "namespaces to accounts that match a label only with namespace missing label (default account not set)",
			args: args{
				defaultAccount: "admin",
				namespaces:     append(TestNamespaces, TestNamespace5),
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "",
					IgnoreMissingLabel: false,
				},
			},
			want: map[string][]inventory.Namespace{
				"account1": {TestNamespace1},
				"account2": {TestNamespace2},
				"account3": {TestNamespace3},
				"account4": {TestNamespace4},
				"admin":    {TestNamespace5},
			},
		},
		{
			name: "namespaces to accounts that match a label only with namespace missing label (default account set)",
			args: args{
				defaultAccount: "admin",
				namespaces:     append(TestNamespaces, TestNamespace5),
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "defaultoverride",
					IgnoreMissingLabel: false,
				},
			},
			want: map[string][]inventory.Namespace{
				"account1":        {TestNamespace1},
				"account2":        {TestNamespace2},
				"account3":        {TestNamespace3},
				"account4":        {TestNamespace4},
				"defaultoverride": {TestNamespace5},
			},
		},
		{
			name: "namespaces to accounts that match a label only with namespace missing label set to ignore",
			args: args{
				defaultAccount: "admin",
				namespaces:     append(TestNamespaces, TestNamespace5),
				accountRoutes:  config.AccountRoutes{},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "",
					IgnoreMissingLabel: true,
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
			name: "mix of account routes and label routing",
			args: args{
				defaultAccount: "admin",
				namespaces:     TestNamespaces,
				accountRoutes: config.AccountRoutes{
					"explicitaccount1": config.AccountRouteDetails{
						Namespaces: []string{"ns1"},
					},
				},
				namespaceLabelRouting: config.AccountRouteByNamespaceLabel{
					LabelKey:           "anchore.io/account",
					DefaultAccount:     "default",
					IgnoreMissingLabel: false,
				},
			},
			want: map[string][]inventory.Namespace{
				"explicitaccount1": {TestNamespace1},
				"account2":         {TestNamespace2},
				"account3":         {TestNamespace3},
				"account4":         {TestNamespace4},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAccountRoutedNamespaces(tt.args.defaultAccount, tt.args.namespaces, tt.args.accountRoutes, tt.args.namespaceLabelRouting)
			assert.Equal(t, tt.want, got)
		})
	}
}
