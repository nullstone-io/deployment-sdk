package s3

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/artifacts"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
)

func NewPusher(logger *log.Logger, nsConfig api.Config, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}
	return &Pusher{
		Logger:   logger,
		NsConfig: nsConfig,
		Infra:    outs,
	}, nil
}

type Pusher struct {
	Logger   *log.Logger
	NsConfig api.Config
	Infra    Outputs
}

func (p Pusher) Push(ctx context.Context, source, version string) error {
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

	p.Logger.Printf("Uploading %s to s3 bucket %s...\n", source, p.Infra.BucketName)
	if err := UploadArtifact(ctx, p.Infra, source, filepaths, version); err != nil {
		return fmt.Errorf("error uploading artifact: %w", err)
	}

	return nil
}
