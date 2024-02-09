package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"net/http"
	"net/url"
)

type ListRemoteTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func ListRemoteTags(ctx context.Context, targetUrl ImageUrl, targetAuth types.AuthConfig) ([]string, error) {
	client := &http.Client{Transport: AuthedTransport{Auth: targetAuth}}
	u := url.URL{
		Scheme: targetUrl.Scheme(),
		Host:   targetUrl.Registry,
		Path:   fmt.Sprintf("/v2/%s/tags/list", targetUrl.RepoName()),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating list remote tags request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting list remote tags response: %w", err)
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	decoder := json.NewDecoder(res.Body)
	var result ListRemoteTagsResponse
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("error reading json response: %w", err)
	}

	return result.Tags, nil
}
