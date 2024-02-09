package s3

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/artifacts"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDirPusher(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
	if err != nil {
		return nil, err
	}
	return &DirPusher{
		OsWriters: osWriters,
		Source:    source,
		Infra:     outs,
	}, nil
}

type DirPusher struct {
	OsWriters logging.OsWriters
	Source    outputs.RetrieverSource
	Infra     Outputs
}

func (p DirPusher) Push(ctx context.Context, source, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()

	if source == "" {
		return fmt.Errorf("no source specified, source artifact (directory or achive) is required to push")
	}
	if version == "" {
		return fmt.Errorf("no version specified, version is required to push")
	}

	filepaths, err := artifacts.WalkDir(source)
	if err != nil {
		return fmt.Errorf("error scanning source: %w", err)
	}

	fmt.Fprintf(stdout, "Uploading %s to s3 bucket %s...\n", source, p.Infra.ArtifactsBucketName)
	if err := UploadDirArtifact(ctx, p.Infra, source, filepaths, version); err != nil {
		return fmt.Errorf("error uploading artifact: %w", err)
	}

	return nil
}

func (p DirPusher) ListArtifacts(ctx context.Context) ([]string, error) {
	return []string{}, nil
}
