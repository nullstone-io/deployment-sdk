package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/moby/moby/client"
)

// The docker CLI keeps the connection settings for each `docker context` on disk.
// We read that store directly so we talk to the same daemon that `docker` would
// (colima, Rancher Desktop, remote engines) without depending on github.com/docker/cli.
//
// Layout, mirroring github.com/docker/cli/cli/context/store:
//
//	<config>/config.json                                   "currentContext"
//	<config>/contexts/meta/<id>/meta.json                  endpoint host
//	<config>/contexts/tls/<id>/docker/{ca,cert,key}.pem    endpoint tls material
//
// where <id> is the hex-encoded sha256 of the context name.
const (
	envOverrideConfigDir = "DOCKER_CONFIG"
	envOverrideContext   = "DOCKER_CONTEXT"

	defaultContextName = "default"
	dockerEndpointName = "docker"
)

// Endpoint describes how to reach the docker daemon.
// A blank Host means the platform default socket.
type Endpoint struct {
	Host          string
	SkipTLSVerify bool
	CAFile        string
	CertFile      string
	KeyFile       string
}

// ResolveEndpoint reports which daemon the docker CLI would talk to, using the same
// precedence: DOCKER_HOST, then DOCKER_CONTEXT, then the current context in config.json.
// DOCKER_HOST wins over any context, and carries its own TLS config through
// DOCKER_CERT_PATH/DOCKER_TLS_VERIFY, so it resolves to an empty Endpoint here.
func ResolveEndpoint() (Endpoint, error) {
	if host := os.Getenv(client.EnvOverrideHost); host != "" {
		return Endpoint{}, nil
	}

	name := os.Getenv(envOverrideContext)
	if name == "" {
		var err error
		if name, err = currentContextName(); err != nil {
			return Endpoint{}, err
		}
	}
	if name == "" || name == defaultContextName {
		return Endpoint{}, nil
	}
	return loadContextEndpoint(name)
}

// DockerConfigDir is the directory holding config.json and the context store.
func DockerConfigDir() string {
	if dir := os.Getenv(envOverrideConfigDir); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".docker")
}

// currentContextName reads "currentContext" from config.json.
// A missing or unreadable config file just means the default context.
func currentContextName() (string, error) {
	dir := DockerConfigDir()
	if dir == "" {
		return "", nil
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("error reading docker config: %w", err)
	}
	var config struct {
		CurrentContext string `json:"currentContext"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", fmt.Errorf("error parsing docker config: %w", err)
	}
	return config.CurrentContext, nil
}

func loadContextEndpoint(name string) (Endpoint, error) {
	dir := DockerConfigDir()
	if dir == "" {
		return Endpoint{}, fmt.Errorf("docker context %q not found: unable to locate docker config directory", name)
	}
	contextDir := contextDirOf(name)

	raw, err := os.ReadFile(filepath.Join(dir, "contexts", "meta", contextDir, "meta.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Endpoint{}, fmt.Errorf("docker context %q not found", name)
		}
		return Endpoint{}, fmt.Errorf("error reading docker context %q: %w", name, err)
	}
	var meta struct {
		Endpoints map[string]struct {
			Host          string `json:"Host"`
			SkipTLSVerify bool   `json:"SkipTLSVerify"`
		} `json:"Endpoints"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return Endpoint{}, fmt.Errorf("error parsing docker context %q: %w", name, err)
	}
	docker, ok := meta.Endpoints[dockerEndpointName]
	if !ok {
		return Endpoint{}, fmt.Errorf("docker context %q does not have a docker endpoint", name)
	}

	endpoint := Endpoint{Host: docker.Host, SkipTLSVerify: docker.SkipTLSVerify}
	tlsDir := filepath.Join(dir, "contexts", "tls", contextDir, dockerEndpointName)
	endpoint.CAFile = existingFile(filepath.Join(tlsDir, "ca.pem"))
	endpoint.CertFile = existingFile(filepath.Join(tlsDir, "cert.pem"))
	endpoint.KeyFile = existingFile(filepath.Join(tlsDir, "key.pem"))
	return endpoint, nil
}

// contextDirOf mirrors store.contextdirOf: the sha256 of the context name, hex-encoded.
func contextDirOf(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

// existingFile returns filename if it exists, otherwise a blank string.
// TLS material is optional: contexts pointing at a local socket carry none.
func existingFile(filename string) string {
	if _, err := os.Stat(filename); err != nil {
		return ""
	}
	return filename
}
