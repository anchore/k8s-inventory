package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_processAnnotations(t *testing.T) {
	type args struct {
		annotationsOrLabels map[string]string
		include             []string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Empty include",
			args: args{
				annotationsOrLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				include: []string{},
			},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "Include explicit keys",
			args: args{
				annotationsOrLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value2",
				},
				include: []string{"key1", "key3"},
			},
			want: map[string]string{
				"key1": "value1",
				"key3": "value2",
			},
		},
		{
			name: "Include non-existent keys",
			args: args{
				annotationsOrLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value2",
				},
				include: []string{"key1", "key4"},
			},
			want: map[string]string{
				"key1": "value1",
			},
		},
		{
			name: "Empty annotationsOrLabels",
			args: args{
				annotationsOrLabels: map[string]string{},
				include:             []string{"key1", "key4"},
			},
			want: map[string]string{},
		},
		{
			name: "Include keys by regex pattern",
			args: args{
				annotationsOrLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value2",
				},
				include: []string{"key[1-2]"},
			},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "Include keys by regex pattern (all)",
			args: args{
				annotationsOrLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value2",
				},
				include: []string{".*"},
			},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value2",
			},
		},
		{
			name: "Include keys by regex pattern (non-existent)",
			args: args{
				annotationsOrLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value2",
				},
				include: []string{"key[4-5]"},
			},
			want: map[string]string{},
		},
		{
			name: "Include keys by regex pattern and explicit",
			args: args{
				annotationsOrLabels: map[string]string{
					"key1":     "value1",
					"key2":     "value2",
					"key3":     "value2",
					"explicit": "value2",
				},
				include: []string{"key[1-2]", "explicit"},
			},
			want: map[string]string{
				"key1":     "value1",
				"key2":     "value2",
				"explicit": "value2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processAnnotationsOrLabels(tt.args.annotationsOrLabels, tt.args.include)
			assert.Equal(t, tt.want, got)
		})
	}
}
