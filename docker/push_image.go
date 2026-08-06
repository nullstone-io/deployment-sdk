package docker

import (
	"context"
	"fmt"

	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

func PushImage(ctx context.Context, dockerCli *command.DockerCli, targetUrl ImageUrl, targetAuth registry.AuthConfig) error {
	encodedAuth, err := EncodeAuthToBase64(targetAuth)
	if err != nil {
		return fmt.Errorf("error encoding remote auth configuration: %w", err)
	}
	options := client.ImagePushOptions{
		All:          false,
		RegistryAuth: encodedAuth,
	}

	responseBody, err := dockerCli.Client().ImagePush(ctx, targetUrl.String(), options)
	if err != nil {
		return err
	}

	errStream := dockerCli.Err()
	return jsonmessage.DisplayJSONMessagesStream(responseBody, errStream, errStream.FD(), errStream.IsTerminal(), nil)
}
