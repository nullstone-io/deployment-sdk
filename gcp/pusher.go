package gcp

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/gcp/gar"
	"github.com/nullstone-io/deployment-sdk/gcp/gcr"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"strings"
)

type Outputs struct {
	ImageRepoUrl docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher  ServiceAccount  `ns:"image_pusher,optional"`
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

var _ app.Pusher = &Pusher{}

type Pusher struct {
	OsWriters logging.OsWriters
	Infra     Outputs
}

func (p Pusher) Push(ctx context.Context, source, version string) error {
	pusher := p.getInfraSpecificPusher()
	return pusher.Push(ctx, source, version)
}

func (p Pusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	pusher := p.getInfraSpecificPusher()
	return pusher.ListArtifactVersions(ctx)
}

func (p Pusher) getInfraSpecificPusher() app.Pusher {
	targetUrl := p.Infra.ImageRepoUrl
	// the new google artifact registry has a url with docker.pkg.dev in it
	// otherwise we fall back to the old gcr.io registry
	if strings.Contains(targetUrl.Registry, "docker.pkg.dev") {
		return gar.Pusher{
			OsWriters: p.OsWriters,
			Infra:     p.Infra,
		}
	}
	return gcr.Pusher{
		OsWriters: p.OsWriters,
		Infra:     p.Infra,
	}
}
