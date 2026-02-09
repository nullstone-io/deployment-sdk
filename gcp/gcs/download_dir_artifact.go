package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

func DownloadDirArtifact(ctx context.Context, infra Outputs, localDir string, version string) error {
	prefix := infra.ArtifactsKey(version)
	prefix = strings.TrimPrefix(prefix, "/")

	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadScopes...)
	if err != nil {
		return fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(infra.ArtifactsBucketName)
	logger := log.New(os.Stderr, "", 0)

	query := &storage.Query{Prefix: prefix}
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("error listing bucket objects: %w", err)
		}

		relPath := strings.TrimPrefix(attrs.Name, prefix)
		if relPath == "" || strings.HasSuffix(relPath, "/") {
			continue
		}

		localPath := filepath.Join(localDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("error creating directory for %q: %w", localPath, err)
		}

		reader, err := bucket.Object(attrs.Name).NewReader(ctx)
		if err != nil {
			return fmt.Errorf("error downloading %q: %w", attrs.Name, err)
		}

		file, err := os.Create(localPath)
		if err != nil {
			reader.Close()
			return fmt.Errorf("error creating local file %q: %w", localPath, err)
		}
		_, err = io.Copy(file, reader)
		reader.Close()
		file.Close()
		if err != nil {
			return fmt.Errorf("error writing %q: %w", localPath, err)
		}
		logger.Println(fmt.Sprintf("Downloaded %s", attrs.Name))
	}

	return nil
}
