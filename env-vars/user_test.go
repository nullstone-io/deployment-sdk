package env_vars

import (
	"testing"

	"github.com/nullstone-io/deployment-sdk/app"
)

func TestResolveUser(t *testing.T) {
	tests := []struct {
		name string
		meta app.DeployMetadata
		want map[string]string
	}{
		{
			name: "nil env vars",
			meta: app.DeployMetadata{},
			want: map[string]string{},
		},
		{
			name: "literal values pass through",
			meta: app.DeployMetadata{EnvVars: map[string]string{"FOO": "bar"}},
			want: map[string]string{"FOO": "bar"},
		},
		{
			name: "interpolate against another user env var",
			meta: app.DeployMetadata{EnvVars: map[string]string{
				"BASE": "hello",
				"FULL": "{{ BASE }}-world",
			}},
			want: map[string]string{
				"BASE": "hello",
				"FULL": "hello-world",
			},
		},
		{
			name: "interpolate without surrounding spaces",
			meta: app.DeployMetadata{EnvVars: map[string]string{
				"BASE": "hello",
				"FULL": "{{BASE}}-world",
			}},
			want: map[string]string{
				"BASE": "hello",
				"FULL": "hello-world",
			},
		},
		{
			name: "interpolate against standard env var",
			meta: app.DeployMetadata{
				Version: "v1.2.3",
				EnvVars: map[string]string{"TAG": "release-{{ NULLSTONE_VERSION }}"},
			},
			want: map[string]string{"TAG": "release-v1.2.3"},
		},
		{
			name: "unknown reference left as literal",
			meta: app.DeployMetadata{EnvVars: map[string]string{"FOO": "a-{{ MISSING }}-b"}},
			want: map[string]string{"FOO": "a-{{ MISSING }}-b"},
		},
		{
			name: "transitive references resolve",
			meta: app.DeployMetadata{EnvVars: map[string]string{
				"A": "1",
				"B": "{{ A }}2",
				"C": "{{ B }}3",
			}},
			want: map[string]string{"A": "1", "B": "12", "C": "123"},
		},
		{
			name: "self reference left as literal",
			meta: app.DeployMetadata{EnvVars: map[string]string{"A": "x{{ A }}y"}},
			want: map[string]string{"A": "x{{ A }}y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveUser(tt.meta)
			if len(got) != len(tt.want) {
				t.Fatalf("ResolveUser() = %v, want %v", got, tt.want)
			}
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("ResolveUser()[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}
