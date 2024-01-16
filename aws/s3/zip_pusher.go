package s3

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"os"
)

func NewZipPusher(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
	if err != nil {
		return nil, err
	}
	return &ZipPusher{
		OsWriters: osWriters,
		Infra:     outs,
	}, nil
}

type ZipPusher struct {
	OsWriters logging.OsWriters
	Infra     Outputs
}

func (p ZipPusher) Push(ctx context.Context, source, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()

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

	fmt.Fprintf(stdout, "Uploading %s to artifacts bucket\n", p.Infra.ArtifactsKey(version))
	if err := UploadZipArtifact(ctx, p.Infra, file, version); err != nil {
		return fmt.Errorf("error uploading artifact: %w", err)
	}

	fmt.Fprintln(stdout, "Upload complete")
	return nil
}
