package docker

import (
	"context"
	"github.com/docker/docker/api/types"
)

type ListRemoteTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func ListRemoteTags(ctx context.Context, targetUrl ImageUrl, targetAuth types.AuthConfig) ([]string, error) {
	return []string{}, nil
}
