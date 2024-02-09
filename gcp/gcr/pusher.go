package gcr

import (
	"context"
	"fmt"
	dockertypes "github.com/docker/docker/api/types"
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

func NewPusher(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
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

func (p Pusher) Push(ctx context.Context, source, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	// TODO: Log information to logger

	sourceUrl := docker.ParseImageUrl(source)
	targetUrl := p.Infra.ImageRepoUrl
	targetUrl.Tag = version

	if err := p.validate(targetUrl); err != nil {
		return err
	}

	targetAuth, err := p.getGcrLoginAuth(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving image registry credentials: %w", err)
	}

	dockerCli, err := docker.DiscoverDockerCli(p.OsWriters)
	if err != nil {
		return fmt.Errorf("error creating docker client: %w", err)
	}

	fmt.Fprintf(stdout, "Retagging %s => %s\n", sourceUrl.String(), targetUrl.String())
	if err := dockerCli.Client().ImageTag(ctx, sourceUrl.String(), targetUrl.String()); err != nil {
		return fmt.Errorf("error retagging image: %w", err)
	}

	fmt.Fprintf(stdout, "Pushing %s\n", targetUrl.String())
	if err := docker.PushImage(ctx, dockerCli, targetUrl, targetAuth); err != nil {
		return fmt.Errorf("error pushing image: %w", err)
	}

	return nil
}

func (p Pusher) CalculateVersion(ctx context.Context, commitSha string) (string, error) {
	return commitSha, nil
}

func (p Pusher) validate(targetUrl docker.ImageUrl) error {
	if targetUrl.String() == "" {
		return fmt.Errorf("cannot push if 'image_repo_url' module output is missing")
	}
	if targetUrl.Tag == "" {
		return fmt.Errorf("no version was specified, version is required to push image")
	}
	if !strings.Contains(targetUrl.Registry, "gcr.io") {
		return fmt.Errorf("this app only supports push to GCP GCR (image=%s)", targetUrl)
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
