package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		currentContext string
		env            map[string]string
		want           Endpoint
		wantErr        string
	}{
		{
			name:           "DOCKER_HOST takes precedence over the current context",
			currentContext: "colima",
			env:            map[string]string{"DOCKER_HOST": "tcp://1.2.3.4:2376"},
			want:           Endpoint{},
		},
		{
			name:           "DOCKER_CONTEXT takes precedence over the current context",
			currentContext: "default",
			env:            map[string]string{"DOCKER_CONTEXT": "colima"},
			want:           Endpoint{Host: "unix:///home/nullstone/.colima/default/docker.sock"},
		},
		{
			name:           "current context from config.json",
			currentContext: "colima",
			want:           Endpoint{Host: "unix:///home/nullstone/.colima/default/docker.sock"},
		},
		{
			name:           "default context uses the platform default socket",
			currentContext: "default",
			want:           Endpoint{},
		},
		{
			name:           "missing currentContext uses the platform default socket",
			currentContext: "",
			want:           Endpoint{},
		},
		{
			name:           "remote context carries tls material",
			currentContext: "remote",
			want: Endpoint{
				Host:          "tcp://docker.nullstone.io:2376",
				SkipTLSVerify: true,
			},
		},
		{
			name:           "unknown context",
			currentContext: "nope",
			wantErr:        `docker context "nope" not found`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configDir := setupContextStore(t, test.currentContext)
			t.Setenv("DOCKER_CONFIG", configDir)
			t.Setenv("DOCKER_HOST", "")
			t.Setenv("DOCKER_CONTEXT", "")
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			got, err := ResolveEndpoint()
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)

			want := test.want
			if want.Host == "tcp://docker.nullstone.io:2376" {
				// The tls material is written into a temp dir, so fill in the paths here.
				tlsDir := filepath.Join(configDir, "contexts", "tls", contextDirOf("remote"), "docker")
				want.CAFile = filepath.Join(tlsDir, "ca.pem")
				want.CertFile = filepath.Join(tlsDir, "cert.pem")
				want.KeyFile = filepath.Join(tlsDir, "key.pem")
			}
			assert.Equal(t, want, got)
		})
	}
}

// setupContextStore lays out a docker config directory containing a "colima" context
// (local socket, no tls) and a "remote" context (tcp endpoint with tls material).
func setupContextStore(t *testing.T, currentContext string) string {
	t.Helper()
	configDir := t.TempDir()

	writeContext(t, configDir, "colima", map[string]any{
		"Name": "colima",
		"Endpoints": map[string]any{
			"docker": map[string]any{
				"Host":          "unix:///home/nullstone/.colima/default/docker.sock",
				"SkipTLSVerify": false,
			},
		},
	})
	writeContext(t, configDir, "remote", map[string]any{
		"Name": "remote",
		"Endpoints": map[string]any{
			"docker": map[string]any{
				"Host":          "tcp://docker.nullstone.io:2376",
				"SkipTLSVerify": true,
			},
		},
	})

	tlsDir := filepath.Join(configDir, "contexts", "tls", contextDirOf("remote"), "docker")
	require.NoError(t, os.MkdirAll(tlsDir, 0755))
	for _, filename := range []string{"ca.pem", "cert.pem", "key.pem"} {
		require.NoError(t, os.WriteFile(filepath.Join(tlsDir, filename), []byte("test"), 0600))
	}

	if currentContext != "" {
		raw, err := json.Marshal(map[string]any{"currentContext": currentContext})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), raw, 0600))
	}
	return configDir
}

func writeContext(t *testing.T, configDir string, name string, meta map[string]any) {
	t.Helper()
	metaDir := filepath.Join(configDir, "contexts", "meta", contextDirOf(name))
	require.NoError(t, os.MkdirAll(metaDir, 0755))
	raw, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(metaDir, "meta.json"), raw, 0600))
}
