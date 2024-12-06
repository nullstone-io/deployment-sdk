package gcr

import (
	"context"
	"fmt"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"strings"
)

var (
	GCRScopes = []string{"https://www.googleapis.com/auth/cloud-platform"}
)

type Outputs struct {
	ImageRepoUrl docker.ImageUrl    `ns:"image_repo_url,optional"`
	ImagePusher  gcp.ServiceAccount `ns:"image_pusher,optional"`
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
	colorstring.Fprintln(stdout, "[bold]Retrieved GCR outputs")
	fmt.Fprintf(stdout, "	image_repo_url: %s\n", p.Infra.ImageRepoUrl)
	fmt.Fprintf(stdout, "	image_pusher:   %s\n", p.Infra.ImagePusher.Email)
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

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Authenticating with GCR...")
	targetAuth, err := p.getGcrLoginAuth(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving image registry credentials: %w", err)
	}
	fmt.Fprintln(stdout, "Authenticated")

	dockerCli, err := docker.DiscoverDockerCli(p.OsWriters)
	if err != nil {
		return fmt.Errorf("error creating docker client: %w", err)
	}

	fmt.Fprintf(stdout, "Retagging source image %s => %s\n", sourceUrl, targetUrl)
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
	targetAuth, err := p.getGcrLoginAuth(ctx)
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
	if !strings.Contains(targetUrl.Registry, "gcr.io") &&
		!strings.Contains(targetUrl.Registry, "docker.pkg.dev") {
		return fmt.Errorf("this app only supports push to GCP and GAP GCR (image=%s)", targetUrl)
	}
	// NOTE: For now, we are assuming that the production docker image is hosted in GCR
	// This will likely need to be refactored to support pushing to other image registries
	if p.Infra.ImagePusher.PrivateKey == "" {
		return fmt.Errorf("cannot push without an authorized user, make sure 'image_pusher' output is not empty")
	}

	return nil
}

func (p Pusher) getGcrLoginAuth(ctx context.Context) (dockertypes.AuthConfig, error) {
	ts, err := p.Infra.ImagePusher.TokenSource(ctx, GCRScopes...)
	if err != nil {
		return dockertypes.AuthConfig{}, fmt.Errorf("error creating access token source: %w", err)
	}
	token, err := ts.Token()
	if err != nil {
		return dockertypes.AuthConfig{}, fmt.Errorf("error retrieving access token: %w", err)
	}
	if token == nil || token.AccessToken == "" {
		return dockertypes.AuthConfig{}, nil
	}

	serverAddr := p.Infra.ImageRepoUrl.Registry
	if serverAddr == "" {
		serverAddr = "gcr.io"
	}
	return dockertypes.AuthConfig{
		ServerAddress: serverAddr,
		Username:      "oauth2accesstoken",
		Password:      token.AccessToken,
	}, nil
}
