package blob

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
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

	client, err := azblob.NewClient(p.Infra.BlobEndpoint(), &p.Infra.Deployer, nil)
	if err != nil {
		return fmt.Errorf("error creating blob client: %w", err)
	}

	objectKey := p.Infra.ArtifactsKey(version)
	objectKey = strings.TrimPrefix(objectKey, "/")

	fmt.Fprintf(stdout, "Uploading %s to Azure Blob Storage...\n", source)
	file, err := os.Open(absSource)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file %q does not exist", source)
		}
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer file.Close()

	_, err = client.UploadFile(ctx, p.Infra.ContainerName, objectKey, file, nil)
	if err != nil {
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

	client, err := azblob.NewClient(p.Infra.BlobEndpoint(), &p.Infra.Deployer, nil)
	if err != nil {
		return fmt.Errorf("error creating blob client: %w", err)
	}

	objectKey := p.Infra.ArtifactsKey(version)
	objectKey = strings.TrimPrefix(objectKey, "/")
	localPath := fmt.Sprintf("./%s-%s-%s.zip", p.AppDetails.App.Name, p.AppDetails.Env.Name, version)

	fmt.Fprintf(stdout, "Downloading %s from Azure Blob Storage...\n", objectKey)
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("error creating local file: %w", err)
	}
	defer file.Close()

	_, err = client.DownloadFile(ctx, p.Infra.ContainerName, objectKey, file, nil)
	if err != nil {
		return fmt.Errorf("error downloading artifact: %w", err)
	}

	fmt.Fprintln(stdout, "Download complete")
	return nil
}

func (p ZipPusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	client, err := azblob.NewClient(p.Infra.BlobEndpoint(), &p.Infra.Deployer, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating blob client: %w", err)
	}

	results := make([]string, 0)
	if before, after, found := strings.Cut(p.Infra.ArtifactsKeyTemplate, KeyTemplateAppVersion); found {
		pager := client.NewListBlobsFlatPager(p.Infra.ContainerName, &azblob.ListBlobsFlatOptions{
			Prefix: &before,
		})
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("error listing blobs: %w", err)
			}
			for _, blob := range page.Segment.BlobItems {
				if blob.Name == nil {
					continue
				}
				cur := strings.TrimPrefix(*blob.Name, before)
				if after != "" {
					cur = strings.TrimSuffix(cur, after)
				}
				results = append(results, cur)
			}
		}
	} else {
		pager := client.NewListBlobsFlatPager(p.Infra.ContainerName, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("error listing blobs: %w", err)
			}
			for _, blob := range page.Segment.BlobItems {
				if blob.Name == nil {
					continue
				}
				results = append(results, *blob.Name)
			}
		}
	}

	return results, nil
}
