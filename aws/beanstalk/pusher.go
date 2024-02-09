package beanstalk

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/s3"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewPusher(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
	if err != nil {
		return nil, err
	}
	zipPusher := &s3.ZipPusher{
		OsWriters: osWriters,
		Infra: s3.Outputs{
			Region:               outs.Region,
			Deployer:             outs.Deployer,
			ArtifactsBucketName:  outs.ArtifactsBucketName,
			ArtifactsKeyTemplate: outs.ArtifactsKeyTemplate,
		},
	}

	return &Pusher{
		zipPusher: zipPusher,
		OsWriters: osWriters,
		Infra:     outs,
	}, nil
}

type Pusher struct {
	zipPusher *s3.ZipPusher
	OsWriters logging.OsWriters
	Infra     Outputs
}

func (p Pusher) Push(ctx context.Context, source, version string) error {
	if err := p.zipPusher.Push(ctx, source, version); err != nil {
		return err
	}

	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()
	fmt.Fprintf(stdout, "Creating application version %q...\n", version)
	if _, err := CreateAppVersion(ctx, p.Infra, version); err != nil {
		return fmt.Errorf("error creating application version: %w", err)
	}
	fmt.Fprintf(stdout, "Created application version %q\n", version)

	return nil
}

func (p Pusher) ListArtifacts(ctx context.Context) ([]string, error) {
	return p.zipPusher.ListArtifacts(ctx)
}
