package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/anchore/k8s-inventory/pkg/client"
)

func Test_fetchNamespaces(t *testing.T) {
	type args struct {
		c         client.Client
		batchSize int64
		timeout   int64
		excludes  []string
		includes  []string
	}
	tests := []struct {
		name    string
		args    args
		want    []Namespace
		wantErr bool
	}{
		{
			name: "successfully returns namespaces",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(&v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-namespace",
							UID:  "test-uid",
							Annotations: map[string]string{
								"test-annotation": "test-value",
							},
							Labels: map[string]string{
								"test-label": "test-value",
							},
						},
					}),
				},
				batchSize: 100,
				timeout:   10,
				excludes:  []string{},
				includes:  []string{},
			},
			want: []Namespace{
				{
					Name:        "test-namespace",
					UID:         "test-uid",
					Annotations: map[string]string{"test-annotation": "test-value"},
					Labels:      map[string]string{"test-label": "test-value"},
				},
			},
		},
		{
			name: "returns nil when no namespaces are found",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(),
				},
				batchSize: 100,
				timeout:   10,
				excludes:  []string{},
				includes:  []string{},
			},
			want: nil,
		},
		{
			name: "successfully excludes namespaces",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
								UID:  "test-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						},
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "excluded-namespace",
								UID:  "test-excluded-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						}),
				},
				batchSize: 100,
				timeout:   10,
				excludes:  []string{"excluded-namespace"},
				includes:  []string{},
			},
			want: []Namespace{
				{
					Name:        "test-namespace",
					UID:         "test-uid",
					Annotations: map[string]string{"test-annotation": "test-value"},
					Labels:      map[string]string{"test-label": "test-value"},
				},
			},
		},
		{
			name: "successfully excludes namespaces by regex",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
								UID:  "test-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						},
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "excluded-namespace",
								UID:  "test-excluded-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						},
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "excluded-namespace2",
								UID:  "test-excluded-uid2",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						}),
				},
				batchSize: 100,
				timeout:   10,
				excludes:  []string{"excluded.*"},
				includes:  []string{},
			},
			want: []Namespace{
				{
					Name:        "test-namespace",
					UID:         "test-uid",
					Annotations: map[string]string{"test-annotation": "test-value"},
					Labels:      map[string]string{"test-label": "test-value"},
				},
			},
		},
		{
			name: "successfully shows only explicitly included namespaces",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
								UID:  "test-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						},
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "excluded-namespace",
								UID:  "test-excluded-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						},
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "excluded-namespace2",
								UID:  "test-excluded-uid2",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						}),
				},
				batchSize: 100,
				timeout:   10,
				excludes:  []string{"exclude.*"},
				includes:  []string{"test-namespace"},
			},
			want: []Namespace{
				{
					Name:        "test-namespace",
					UID:         "test-uid",
					Annotations: map[string]string{"test-annotation": "test-value"},
					Labels:      map[string]string{"test-label": "test-value"},
				},
			},
		},
		{
			name: "successfully shows only explicitly included namespaces when excludes are also set",
			args: args{
				c: client.Client{
					Clientset: fake.NewSimpleClientset(
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
								UID:  "test-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						},
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "excluded-namespace",
								UID:  "test-excluded-uid",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						},
						&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "excluded-namespace2",
								UID:  "test-excluded-uid2",
								Annotations: map[string]string{
									"test-annotation": "test-value",
								},
								Labels: map[string]string{
									"test-label": "test-value",
								},
							},
						}),
				},
				batchSize: 100,
				timeout:   10,
				excludes:  []string{},
				includes:  []string{"test-namespace"},
			},
			want: []Namespace{
				{
					Name:        "test-namespace",
					UID:         "test-uid",
					Annotations: map[string]string{"test-annotation": "test-value"},
					Labels:      map[string]string{"test-label": "test-value"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchNamespaces(
				tt.args.c,
				tt.args.batchSize,
				tt.args.timeout,
				tt.args.excludes,
				tt.args.includes,
			)
			if (err != nil) != tt.wantErr {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
