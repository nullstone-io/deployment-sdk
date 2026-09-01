package ecr

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/mitchellh/colorstring"
	dockerregistry "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/aws/iampropagation"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	Region       string            `ns:"region"`
	ImageRepoUrl docker.ImageUrl   `ns:"image_repo_url,optional"`
	ImagePusher  nsaws.IamIdentity `ns:"image_pusher,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.BlockId, ws.EnvId)
	o.ImagePusher.RemoteProvider = credsFactory(types.AutomationPurposePush, "image_pusher")
}

func NewPusher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)
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
	stderr := p.OsWriters.Stderr()
	colorstring.Fprintln(stderr, "[bold]Retrieved ECR outputs")
	fmt.Fprintf(stderr, "\tregion:         %s\n", p.Infra.Region)
	fmt.Fprintf(stderr, "\timage_repo_url: %s\n", p.Infra.ImageRepoUrl)
	fmt.Fprintf(stderr, "\timage_pusher:   %s\n", p.Infra.ImagePusher.Name)
}

func (p Pusher) Push(ctx context.Context, source, version string) error {
	stderr := p.OsWriters.Stderr()
	p.Print()

	sourceUrl := docker.ParseImageUrl(source)
	targetUrl := p.Infra.ImageRepoUrl
	targetUrl.Tag = version

	if err := p.validate(targetUrl); err != nil {
		return err
	}

	dockerCli, err := docker.DiscoverDockerCli(p.OsWriters)
	if err != nil {
		return fmt.Errorf("error creating docker client: %w", err)
	}

	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "Retagging source image %s => %s\n", sourceUrl, targetUrl)
	opts := client.ImageTagOptions{Source: sourceUrl.String(), Target: targetUrl.String()}
	if _, err := dockerCli.Client().ImageTag(ctx, opts); err != nil {
		return fmt.Errorf("error retagging image: %w", err)
	}

	// On a first launch, the `image_pusher` identity is created seconds before this push runs.
	// AWS IAM is eventually consistent, so authenticating/pushing can fail with access-denied
	// until the identity's permissions propagate; retry both together until they do.
	return iampropagation.Retry(ctx, p.OsWriters, "pushing the image to ECR", iampropagation.DefaultWindow, func(ctx context.Context) error {
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Authenticating with ECR...")
		targetAuth, err := p.getEcrLoginAuth(ctx)
		if err != nil {
			return fmt.Errorf("error retrieving image registry credentials: %w", err)
		}
		fmt.Fprintln(stderr, "Authenticated")

		colorstring.Fprintf(stderr, "[bold]Pushing docker image to %s\n", targetUrl)
		if err := docker.PushImage(ctx, dockerCli, targetUrl, targetAuth); err != nil {
			return fmt.Errorf("error pushing image: %w", err)
		}
		return nil
	})
}

func (p Pusher) Pull(ctx context.Context, version string) error {
	stderr := p.OsWriters.Stderr()
	p.Print()

	sourceUrl := p.Infra.ImageRepoUrl
	sourceUrl.Tag = version
	if err := p.validate(sourceUrl); err != nil {
		return err
	}

	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Authenticating with ECR...")
	sourceAuth, err := p.getEcrLoginAuth(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving image registry credentials: %w", err)
	}
	fmt.Fprintln(stderr, "Authenticated")

	dockerCli, err := docker.DiscoverDockerCli(p.OsWriters)
	if err != nil {
		return fmt.Errorf("error creating docker client: %w", err)
	}

	fmt.Fprintln(stderr)
	colorstring.Fprintf(stderr, "[bold]Pulling docker image %s\n", sourceUrl)
	if err := docker.PullImage(ctx, dockerCli, sourceUrl, sourceAuth); err != nil {
		return fmt.Errorf("error pulling image: %w", err)
	}

	return nil
}

func (p Pusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	var tags []string
	// Same IAM propagation race as Push: this can run moments after the `image_pusher` identity is created.
	err := iampropagation.Retry(ctx, p.OsWriters, "listing artifact versions in ECR", iampropagation.DefaultWindow, func(ctx context.Context) error {
		targetAuth, err := p.getEcrLoginAuth(ctx)
		if err != nil {
			return fmt.Errorf("error retrieving image registry credentials: %w", err)
		}

		if tags, err = docker.ListRemoteTags(ctx, p.Infra.ImageRepoUrl, targetAuth); err != nil {
			return fmt.Errorf("error listing remote tags: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	if err := p.Infra.ImagePusher.Validate(); err != nil {
		return fmt.Errorf("cannot push without an authorized user specified in `image_pusher`: %w", err)
	}
	return nil
}

func (p Pusher) getEcrLoginAuth(ctx context.Context) (dockerregistry.AuthConfig, error) {
	// Access-denied retries (IAM propagation on first launch) are handled by iampropagation.Retry
	// around the whole push/list operation, so the client uses the default SDK retryer here.
	ecrClient := ecr.NewFromConfig(nsaws.NewConfig(p.Infra.ImagePusher, p.Infra.Region))
	out, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return dockerregistry.AuthConfig{}, err
	}
	if len(out.AuthorizationData) > 0 {
		authData := out.AuthorizationData[0]
		token, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
		if err != nil {
			return dockerregistry.AuthConfig{}, fmt.Errorf("invalid authorization token: %w", err)
		}
		tokens := strings.SplitN(string(token), ":", 2)
		return dockerregistry.AuthConfig{
			Username:      tokens[0],
			Password:      tokens[1],
			ServerAddress: *authData.ProxyEndpoint,
		}, nil
	}
	return dockerregistry.AuthConfig{}, nil
}
