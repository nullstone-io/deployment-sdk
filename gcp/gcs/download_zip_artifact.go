package gcs

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func DownloadZipArtifact(ctx context.Context, infra Outputs, localPath string, version string) error {
	objKey := infra.ArtifactsKey(version)
	objKey = strings.TrimPrefix(objKey, "/")

	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadScopes...)
	if err != nil {
		return fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()

	reader, err := client.Bucket(infra.ArtifactsBucketName).Object(objKey).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error downloading %q: %w", objKey, err)
	}
	defer reader.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("error creating local file %q: %w", localPath, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("error writing %q: %w", localPath, err)
	}

	return nil
}
