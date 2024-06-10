package ecr

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"strings"
	"time"
)

type Outputs struct {
	Region       string          `ns:"region"`
	ImageRepoUrl docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher  nsaws.User      `ns:"image_pusher,optional"`
}

func NewPusher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	return &Pusher{
		OsWriters: osWriters,
		Infra:     outs,
	}, nil
}

type Pusher struct {
	OsWriters logging.OsWriters
	Infra     Outputs
}

func (p Pusher) Print() {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved ECR outputs")
	fmt.Fprintf(stdout, "	region:         %s\n", p.Infra.Region)
	fmt.Fprintf(stdout, "	image_repo_url: %s\n", p.Infra.ImageRepoUrl)
	fmt.Fprintf(stdout, "	image_pusher:   %s\n", p.Infra.ImagePusher.Name)
}

func (p Pusher) Push(ctx context.Context, source, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	p.Print()

	sourceUrl := docker.ParseImageUrl(source)
	targetUrl := p.Infra.ImageRepoUrl
	targetUrl.Tag = version

	if err := p.validate(targetUrl); err != nil {
		return err
	}

	colorstring.Fprintln(stdout, "[bold]Authenticating with ECR...")
	targetAuth, err := p.getEcrLoginAuth(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving image registry credentials: %w", err)
	}

	dockerCli, err := docker.DiscoverDockerCli(p.OsWriters)
	if err != nil {
		return fmt.Errorf("error creating docker client: %w", err)
	}

	colorstring.Fprintf(stdout, "[bold]Retagging source image %s => %s\n", sourceUrl, targetUrl)
	if err := dockerCli.Client().ImageTag(ctx, sourceUrl.String(), targetUrl.String()); err != nil {
		return fmt.Errorf("error retagging image: %w", err)
	}

	fmt.Fprintln(stdout)
	colorstring.Fprintf(stdout, "[bold]Pushing docker image to %s\n", targetUrl)
	if err := docker.PushImage(ctx, dockerCli, targetUrl, targetAuth); err != nil {
		return fmt.Errorf("error pushing image: %w", err)
	}

	return nil
}

func (p Pusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	targetAuth, err := p.getEcrLoginAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving image registry credentials: %w", err)
	}

	tags, err := docker.ListRemoteTags(ctx, p.Infra.ImageRepoUrl, targetAuth)
	if err != nil {
		return nil, fmt.Errorf("error listing remote tags: %w", err)
	}
	return tags, nil
}

func (p Pusher) validate(targetUrl docker.ImageUrl) error {
	if targetUrl.String() == "" {
		return fmt.Errorf("cannot push if 'image_repo_url' module output is missing")
	}
	if targetUrl.Tag == "" {
		return fmt.Errorf("no version was specified, version is required to push image")
	}
	if !strings.Contains(targetUrl.Registry, "ecr") &&
		!strings.Contains(targetUrl.Registry, "amazonaws.com") {
		return fmt.Errorf("this app only supports push to AWS ECR (image=%s)", targetUrl)
	}

	// NOTE: For now, we are assuming that the production docker image is hosted in ECR
	// This will likely need to be refactored to support pushing to other image registries
	if p.Infra.ImagePusher.AccessKeyId == "" {
		return fmt.Errorf("cannot push without an authorized user, make sure 'image_pusher' output is not empty")
	}

	return nil
}

func (p Pusher) getEcrLoginAuth(ctx context.Context) (dockertypes.AuthConfig, error) {
	retryOpts := func(options *ecr.Options) {
		// Set retryer to backoff 0s-20s with max attempts of 5s
		// This has a retry window of 0s-100s
		retryer := retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = 5
			options.MaxBackoff = 20 * time.Second
		})
		options.Retryer = retry.AddWithErrorCodes(retryer, (*ecstypes.AccessDeniedException)(nil).ErrorCode())
	}
	ecrClient := ecr.NewFromConfig(nsaws.NewConfig(p.Infra.ImagePusher, p.Infra.Region), retryOpts)
	out, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return dockertypes.AuthConfig{}, err
	}
	if len(out.AuthorizationData) > 0 {
		authData := out.AuthorizationData[0]
		token, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
		if err != nil {
			return dockertypes.AuthConfig{}, fmt.Errorf("invalid authorization token: %w", err)
		}
		tokens := strings.SplitN(string(token), ":", 2)
		return dockertypes.AuthConfig{
			Username:      tokens[0],
			Password:      tokens[1],
			ServerAddress: *authData.ProxyEndpoint,
		}, nil
	}
	return dockertypes.AuthConfig{}, nil
}
