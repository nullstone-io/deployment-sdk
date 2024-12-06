package gar

import (
	"context"
	"fmt"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/logging"
	"strings"
)

var (
	GARScopes = []string{"https://www.googleapis.com/auth/cloud-platform"}
)

var _ app.Pusher = &Pusher{}

type Pusher struct {
	OsWriters logging.OsWriters
	Infra     gcp.Outputs
}

func (p Pusher) Print() {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	colorstring.Fprintln(stdout, "[bold]Retrieved GAR outputs")
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
	fmt.Fprintln(stdout, "Authenticating with GAR...")
	targetAuth, err := p.getGarLoginAuth(ctx)
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
	targetAuth, err := p.getGarLoginAuth(ctx)
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
	if !strings.Contains(targetUrl.Registry, "docker.pkg.dev") {
		return fmt.Errorf("this app only supports push to GCP GAR (image=%s)", targetUrl)
	}
	if p.Infra.ImagePusher.PrivateKey == "" {
		return fmt.Errorf("cannot push without an authorized user, make sure 'image_pusher' output is not empty")
	}

	return nil
}

func (p Pusher) getGarLoginAuth(ctx context.Context) (dockertypes.AuthConfig, error) {
	ts, err := p.Infra.ImagePusher.TokenSource(ctx, GARScopes...)
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
		serverAddr = "docker.pkg.dev"
	}
	return dockertypes.AuthConfig{
		ServerAddress: serverAddr,
		Username:      "oauth2accesstoken",
		Password:      token.AccessToken,
	}, nil
}
