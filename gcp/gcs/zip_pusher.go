package gcs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewZipPusher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return &ZipPusher{
		OsWriters:  osWriters,
		Source:     source,
		Infra:      outs,
		AppDetails: appDetails,
	}, nil
}

type ZipPusher struct {
	OsWriters  logging.OsWriters
	Source     outputs.RetrieverSource
	Infra      Outputs
	AppDetails app.Details
}

func (p ZipPusher) Push(ctx context.Context, source, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()

	if source == "" {
		return fmt.Errorf("no source specified, source artifact (zip file) is required to push")
	}
	if version == "" {
		return fmt.Errorf("no version specified, version is required to push")
	}
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("error finding absolute filepath to --source=%s: %w", source, err)
	}

	fmt.Fprintf(stdout, "Uploading %s to GCS bucket...\n", source)
	if err := UploadZipArtifact(ctx, p.Infra, absSource, version); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file %q does not exist", source)
		}
		return fmt.Errorf("error uploading artifact: %w", err)
	}

	fmt.Fprintln(stdout, "Upload complete")
	return nil
}

func (p ZipPusher) Pull(ctx context.Context, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()

	if version == "" {
		return fmt.Errorf("no version specified, version is required to pull")
	}

	localPath := fmt.Sprintf("./%s-%s-%s.zip", p.AppDetails.App.Name, p.AppDetails.Env.Name, version)

	fmt.Fprintf(stdout, "Downloading %s from GCS bucket...\n", p.Infra.ArtifactsKey(version))
	if err := DownloadZipArtifact(ctx, p.Infra, localPath, version); err != nil {
		return fmt.Errorf("error downloading artifact: %w", err)
	}

	fmt.Fprintln(stdout, "Download complete")
	return nil
}

func (p ZipPusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	results := make([]string, 0)
	if before, after, found := strings.Cut(p.Infra.ArtifactsKeyTemplate, KeyTemplateAppVersion); found {
		objects, err := ListObjects(ctx, p.Infra, before, "")
		if err != nil {
			return nil, err
		}
		for _, name := range objects {
			version := strings.TrimPrefix(name, before)
			if after != "" {
				version = strings.TrimSuffix(version, after)
			}
			results = append(results, version)
		}
	} else {
		objects, err := ListObjects(ctx, p.Infra, "", "")
		if err != nil {
			return nil, err
		}
		for _, name := range objects {
			results = append(results, name)
		}
	}

	return results, nil
}
