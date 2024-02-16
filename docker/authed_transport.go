package docker

import (
	"encoding/base64"
	"fmt"
	"github.com/docker/docker/api/types"
	"net/http"
)

type AuthedTransport struct {
	Transport http.RoundTripper
	Auth      types.AuthConfig
}

func (t AuthedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if bearer := t.Auth.RegistryToken; bearer != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearer))
	} else if user, pass := t.Auth.Username, t.Auth.Password; user != "" && pass != "" {
		delimited := fmt.Sprintf("%s:%s", user, pass)
		encoded := base64.StdEncoding.EncodeToString([]byte(delimited))
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encoded))
	} else if token := t.Auth.Auth; token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", token))
	}

	if t.Transport == nil {
		return http.DefaultTransport.RoundTrip(req)
	} else {
		return t.Transport.RoundTrip(req)
	}
}
