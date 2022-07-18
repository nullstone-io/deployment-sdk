package docker

import (
	"context"
	"fmt"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
)

func PushImage(ctx context.Context, dockerCli *command.DockerCli, targetUrl ImageUrl, targetAuth types.AuthConfig) error {
	encodedAuth, err := EncodeAuthToBase64(targetAuth)
	if err != nil {
		return fmt.Errorf("error encoding remote auth configuration: %w", err)
	}
	options := types.ImagePushOptions{
		All:          false,
		RegistryAuth: encodedAuth,
	}

	responseBody, err := dockerCli.Client().ImagePush(ctx, targetUrl.String(), options)
	if err != nil {
		return err
	}

	return jsonmessage.DisplayJSONMessagesToStream(responseBody, dockerCli.Out(), nil)
}
