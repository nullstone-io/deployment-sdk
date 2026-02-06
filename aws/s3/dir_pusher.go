package s3

import (
	"context"
	"fmt"
	"strings"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/artifacts"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDirPusher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return &DirPusher{
		OsWriters:  osWriters,
		Source:     source,
		Infra:      outs,
		AppDetails: appDetails,
	}, nil
}

type DirPusher struct {
	OsWriters  logging.OsWriters
	Source     outputs.RetrieverSource
	Infra      Outputs
	AppDetails app.Details
}

func (p DirPusher) Push(ctx context.Context, source, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()

	if source == "" {
		return fmt.Errorf("no source specified, source artifact (directory or archive) is required to push")
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

func (p DirPusher) Pull(ctx context.Context, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()

	if version == "" {
		return fmt.Errorf("no version specified, version is required to pull")
	}

	localDir := fmt.Sprintf("./%s-%s-%s", p.AppDetails.App.Name, p.AppDetails.Env.Name, version)

	fmt.Fprintf(stdout, "Downloading from s3 bucket %s to %s...\n", p.Infra.ArtifactsBucketName, localDir)
	if err := DownloadDirArtifact(ctx, p.Infra, localDir, version); err != nil {
		return fmt.Errorf("error downloading artifact: %w", err)
	}

	fmt.Fprintln(stdout, "Download complete")
	return nil
}

func (p DirPusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	dirs, err := ListDirs(ctx, p.Infra)
	if err != nil {
		return nil, err
	}

	results := make([]string, 0)
	if before, after, found := strings.Cut(p.Infra.ArtifactsKeyTemplate, KeyTemplateAppVersion); found {
		for _, dir := range dirs {
			cur := strings.TrimPrefix(dir, before)
			cur = strings.TrimSuffix(cur, after)
			results = append(results, cur)
		}
	} else {
		for _, dir := range dirs {
			results = append(results, dir)
		}
	}

	return results, nil
}
