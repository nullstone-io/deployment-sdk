package docker

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

func PullImage(ctx context.Context, dockerCli *Cli, sourceUrl ImageUrl, sourceAuth registry.AuthConfig) error {
	encodedAuth, err := EncodeAuthToBase64(sourceAuth)
	if err != nil {
		return fmt.Errorf("error encoding remote auth configuration: %w", err)
	}
	options := client.ImagePullOptions{
		All:          false,
		RegistryAuth: encodedAuth,
	}

	responseBody, err := dockerCli.Client().ImagePull(ctx, sourceUrl.String(), options)
	if err != nil {
		return err
	}

	return jsonmessage.DisplayStream(responseBody, dockerCli.Err())
}
