package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/gcp/creds"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type ServiceAccount struct {
	Email      string `json:"email"`
	PrivateKey string `json:"private_key"`

	RemoteTokenSourcer creds.TokenSourcer `json:"-"`
}

func (a ServiceAccount) TokenSource(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
	// First, try PrivateKey from outputs
	if a.PrivateKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(a.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("service account private key is not base64-encoded: %w", err)
		}
		cfg, err := google.JWTConfigFromJSON(decoded, scopes...)
		if err != nil {
			return nil, fmt.Errorf("unable to read service account credentials json file: %w", err)
		}
		return cfg.TokenSource(ctx), nil
	}

	// If output doesn't have PrivateKey, fall back to remote provider
	if a.RemoteTokenSourcer != nil {
		return a.RemoteTokenSourcer(scopes), nil
	}

	return nil, fmt.Errorf("missing GCP credentials")
}
