package docker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientOptionsTLS connects to a TLS server to verify that the tls material on a
// docker context produces a working https connection, and — just as important — that
// we still reject a daemon we can't verify.
func TestClientOptionsTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Api-Version", "1.51")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := "tcp://" + server.Listener.Addr().String()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	caPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, os.WriteFile(caFile, caPem, 0600))

	tests := []struct {
		name     string
		endpoint Endpoint
		wantErr  string
	}{
		{
			name:     "verifies against the context ca",
			endpoint: Endpoint{Host: host, CAFile: caFile},
		},
		{
			name:     "skip-tls-verify accepts an unverifiable daemon",
			endpoint: Endpoint{Host: host, SkipTLSVerify: true},
		},
		{
			name:     "rejects a daemon the context ca does not vouch for",
			endpoint: Endpoint{Host: host, CAFile: writeUnrelatedCA(t)},
			wantErr:  "certificate",
		},
		{
			name:     "no tls material talks plaintext, matching docker's own behavior",
			endpoint: Endpoint{Host: host},
			wantErr:  "http request to an https server",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts, err := clientOptions(test.endpoint)
			require.NoError(t, err)
			api, err := client.New(opts...)
			require.NoError(t, err)
			defer api.Close()

			_, err = api.Ping(context.Background(), client.PingOptions{})
			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), test.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestClientOptionsMutualTLS verifies we actually present the context's client
// certificate, which a daemon configured for mTLS requires.
func TestClientOptionsMutualTLS(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Api-Version", "1.51")
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	server.StartTLS()
	defer server.Close()

	host := "tcp://" + server.Listener.Addr().String()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	caPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	require.NoError(t, os.WriteFile(caFile, caPem, 0600))
	certFile, keyFile := writeKeyPair(t)

	tests := []struct {
		name     string
		endpoint Endpoint
		wantErr  bool
	}{
		{
			name:     "presents the client certificate",
			endpoint: Endpoint{Host: host, CAFile: caFile, CertFile: certFile, KeyFile: keyFile},
		},
		{
			name:     "presents the client certificate when skipping verification",
			endpoint: Endpoint{Host: host, SkipTLSVerify: true, CertFile: certFile, KeyFile: keyFile},
		},
		{
			name:     "without a client certificate the daemon refuses us",
			endpoint: Endpoint{Host: host, CAFile: caFile},
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts, err := clientOptions(test.endpoint)
			require.NoError(t, err)
			api, err := client.New(opts...)
			require.NoError(t, err)
			defer api.Close()

			_, err = api.Ping(context.Background(), client.PingOptions{})
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestClientOptionsScheme guards the moby client's implicit rule that a tls config is
// what promotes a tcp:// host to https. Losing the tls config would silently downgrade
// us to plaintext against a tls daemon rather than fail loudly.
func TestClientOptionsScheme(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		want     string
	}{
		{
			name:     "plain tcp stays http",
			endpoint: Endpoint{},
			want:     "http",
		},
		{
			name:     "tls material promotes to https",
			endpoint: Endpoint{CAFile: writeUnrelatedCA(t)},
			want:     "https",
		},
		{
			name:     "skip-tls-verify promotes to https",
			endpoint: Endpoint{SkipTLSVerify: true},
			want:     "https",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sniffer := newSchemeSniffer(t)
			test.endpoint.Host = sniffer.host
			opts, err := clientOptions(test.endpoint)
			require.NoError(t, err)
			api, err := client.New(opts...)
			require.NoError(t, err)
			defer api.Close()

			// The sniffer closes the connection after the first byte, so the ping always
			// fails; only the bytes that reached the wire matter.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = api.Ping(ctx, client.PingOptions{})

			assert.Equal(t, test.want, sniffer.scheme(t))
		})
	}
}

// schemeSniffer reports whether a client talks http or https by looking at the first
// byte it sends. The client does not expose its scheme, and the ping error is no good
// as a signal: on Linux the moby client collapses "connection refused" into a generic
// "Cannot connect to the Docker daemon" message that drops the URL entirely.
//
// A TLS handshake always begins with a record of type 0x16; a plaintext HTTP request
// begins with an ASCII method.
type schemeSniffer struct {
	host   string
	result chan string
}

func newSchemeSniffer(t *testing.T) *schemeSniffer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	s := &schemeSniffer{
		host:   "tcp://" + ln.Addr().String(),
		result: make(chan string, 1),
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.sniff(conn)
		}
	}()
	return s
}

// sniff reads one byte, classifies it, and hangs up. Only the first connection is
// reported; any later ones (client retries) are closed without a reading.
func (s *schemeSniffer) sniff(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	first := make([]byte, 1)
	if _, err := io.ReadFull(conn, first); err != nil {
		return
	}
	scheme := "http"
	if first[0] == 0x16 {
		scheme = "https"
	}
	select {
	case s.result <- scheme:
	default:
	}
}

func (s *schemeSniffer) scheme(t *testing.T) string {
	t.Helper()
	select {
	case scheme := <-s.result:
		return scheme
	case <-time.After(5 * time.Second):
		t.Fatal("client never sent a byte to the sniffer")
		return ""
	}
}

// writeUnrelatedCA generates a self-signed CA that has signed nothing. Note that we
// cannot get one from a second httptest server: every httptest TLS server shares the
// same built-in localhost certificate, so it would verify the first server just fine.
func writeUnrelatedCA(t *testing.T) string {
	t.Helper()
	certFile, _ := writeKeyPair(t)
	return certFile
}

// writeKeyPair generates a self-signed certificate and its key, as pem files.
func writeKeyPair(t *testing.T) (certFile string, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "nullstone-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDer, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDer})
	require.NoError(t, os.WriteFile(certFile, certPem, 0600))
	require.NoError(t, os.WriteFile(keyFile, keyPem, 0600))
	return certFile, keyFile
}
