package lambda_zip

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
	"os"
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
		return fmt.Errorf("--source is required to upload artifact")
	}
	if version == "" {
		return fmt.Errorf("--version is required to upload artifact")
	}

	file, err := os.Open(source)
	if os.IsNotExist(err) {
		return fmt.Errorf("source file %q does not exist", source)
	} else if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer file.Close()

	p.Logger.Printf("Uploading %s to artifacts bucket\n", p.Infra.ArtifactsKey(version))
	if err := UploadArtifact(ctx, p.Infra, file, version); err != nil {
		return fmt.Errorf("error uploading artifact: %w", err)
	}

	p.Logger.Printf("Upload complete")
	return nil
}
