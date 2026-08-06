package docker

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/moby/moby/client"
	"github.com/nullstone-io/deployment-sdk/logging"
)

// Cli pairs a docker API client with the streams used to report progress.
type Cli struct {
	api *client.Client
	out io.Writer
	err io.Writer
}

func (c *Cli) Client() *client.Client { return c.api }

func (c *Cli) Out() io.Writer { return c.out }

func (c *Cli) Err() io.Writer { return c.err }

func (c *Cli) Close() error { return c.api.Close() }

// DiscoverDockerCli connects to the same docker daemon that the docker CLI would.
// See ResolveEndpoint for how that daemon is located.
func DiscoverDockerCli(osWriters logging.OsWriters) (*Cli, error) {
	cli := &Cli{out: os.Stdout, err: os.Stderr}
	if osWriters != nil {
		if outWriter := osWriters.Stdout(); outWriter != nil {
			cli.out = outWriter
		}
		if errWriter := osWriters.Stderr(); errWriter != nil {
			cli.err = errWriter
		}
	}

	api, err := NewApiClient()
	if err != nil {
		return nil, err
	}
	cli.api = api
	return cli, nil
}

// NewApiClient creates a docker API client for the current docker context.
func NewApiClient() (*client.Client, error) {
	endpoint, err := ResolveEndpoint()
	if err != nil {
		return nil, err
	}

	opts, err := clientOptions(endpoint)
	if err != nil {
		return nil, err
	}
	// Negotiate so we don't send a newer API version than the daemon understands.
	opts = append(opts, client.WithAPIVersionFromEnv(), client.WithAPIVersionNegotiation())
	return client.New(opts...)
}

func clientOptions(endpoint Endpoint) ([]client.Opt, error) {
	if endpoint.Host == "" {
		// No context to honor: fall back to DOCKER_HOST/DOCKER_CERT_PATH, then the default socket.
		return []client.Opt{client.WithHostFromEnv(), client.WithTLSClientConfigFromEnv()}, nil
	}

	opts := make([]client.Opt, 0)
	if endpoint.SkipTLSVerify {
		// client.WithTLSClientConfig always verifies, so supply the transport ourselves.
		// This must precede WithHost, which configures the dialer on the transport.
		httpClient, err := insecureHttpClient(endpoint)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHTTPClient(httpClient), client.WithHost(endpoint.Host))
		return opts, nil
	}

	opts = append(opts, client.WithHost(endpoint.Host))
	if endpoint.CAFile != "" || endpoint.CertFile != "" {
		opts = append(opts, client.WithTLSClientConfig(endpoint.CAFile, endpoint.CertFile, endpoint.KeyFile))
	}
	return opts, nil
}

func insecureHttpClient(endpoint Endpoint) (*http.Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}
	if endpoint.CertFile != "" && endpoint.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(endpoint.CertFile, endpoint.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("error loading docker context client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, nil
}
