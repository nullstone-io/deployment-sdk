package blob

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
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

	client, err := azblob.NewClient(p.Infra.BlobEndpoint(), &p.Infra.Deployer, nil)
	if err != nil {
		return fmt.Errorf("error creating blob client: %w", err)
	}

	objDir := p.Infra.ArtifactsKey(version)
	objDir = strings.TrimPrefix(objDir, "/")
	logger := log.New(os.Stderr, "", 0)

	fmt.Fprintf(stdout, "Uploading %s to Azure Blob Storage %s/%s...\n", source, p.Infra.StorageAccount, p.Infra.ContainerName)
	for _, fp := range filepaths {
		relPath, err := filepath.Rel(source, fp)
		if err != nil {
			return fmt.Errorf("error computing relative path: %w", err)
		}
		objectKey := strings.Replace(relPath, string(filepath.Separator), "/", -1)
		if objDir != "" {
			objectKey = objDir + "/" + objectKey
		}

		file, err := os.Open(fp)
		if err != nil {
			return fmt.Errorf("error opening file %s: %w", fp, err)
		}
		_, err = client.UploadFile(ctx, p.Infra.ContainerName, objectKey, file, nil)
		file.Close()
		if err != nil {
			return fmt.Errorf("error uploading %s: %w", objectKey, err)
		}
		logger.Printf("Uploaded %s", objectKey)
	}

	return nil
}

func (p DirPusher) Pull(ctx context.Context, version string) error {
	stdout, _ := p.OsWriters.Stdout(), p.OsWriters.Stderr()

	if version == "" {
		return fmt.Errorf("no version specified, version is required to pull")
	}

	client, err := azblob.NewClient(p.Infra.BlobEndpoint(), &p.Infra.Deployer, nil)
	if err != nil {
		return fmt.Errorf("error creating blob client: %w", err)
	}

	prefix := p.Infra.ArtifactsKey(version)
	prefix = strings.TrimPrefix(prefix, "/")
	localDir := fmt.Sprintf("./%s-%s-%s", p.AppDetails.App.Name, p.AppDetails.Env.Name, version)

	fmt.Fprintf(stdout, "Downloading from Azure Blob Storage to %s...\n", localDir)
	pager := client.NewListBlobsFlatPager(p.Infra.ContainerName, &azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("error listing blobs: %w", err)
		}
		for _, blob := range page.Segment.BlobItems {
			if blob.Name == nil {
				continue
			}
			relPath := strings.TrimPrefix(*blob.Name, prefix)
			relPath = strings.TrimPrefix(relPath, "/")
			localPath := filepath.Join(localDir, relPath)

			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}

			resp, err := client.DownloadStream(ctx, p.Infra.ContainerName, *blob.Name, nil)
			if err != nil {
				return fmt.Errorf("error downloading %s: %w", *blob.Name, err)
			}
			file, err := os.Create(localPath)
			if err != nil {
				resp.Body.Close()
				return fmt.Errorf("error creating file %s: %w", localPath, err)
			}
			_, err = file.ReadFrom(resp.Body)
			resp.Body.Close()
			file.Close()
			if err != nil {
				return fmt.Errorf("error writing file %s: %w", localPath, err)
			}
		}
	}

	fmt.Fprintln(stdout, "Download complete")
	return nil
}

func (p DirPusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	client, err := azblob.NewClient(p.Infra.BlobEndpoint(), &p.Infra.Deployer, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating blob client: %w", err)
	}

	results := make([]string, 0)
	if before, after, found := strings.Cut(p.Infra.ArtifactsKeyTemplate, KeyTemplateAppVersion); found {
		delimiter := "/"
		containerClient := client.ServiceClient().NewContainerClient(p.Infra.ContainerName)
		pager := containerClient.NewListBlobsHierarchyPager(delimiter, &container.ListBlobsHierarchyOptions{
			Prefix: &before,
		})
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("error listing blobs: %w", err)
			}
			for _, prefix := range page.Segment.BlobPrefixes {
				if prefix.Name == nil {
					continue
				}
				cur := strings.TrimPrefix(*prefix.Name, before)
				cur = strings.TrimSuffix(cur, "/")
				if after != "" {
					cur = strings.TrimSuffix(cur, after)
				}
				results = append(results, cur)
			}
		}
	}

	return results, nil
}
