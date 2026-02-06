package docker

import (
	"context"
	"fmt"

	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/moby/client"
)

func PullImage(ctx context.Context, dockerCli *command.DockerCli, sourceUrl ImageUrl, sourceAuth registry.AuthConfig) error {
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

	return jsonmessage.DisplayJSONMessagesToStream(responseBody, dockerCli.Out(), nil)
}
