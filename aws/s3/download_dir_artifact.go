package s3

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func DownloadDirArtifact(ctx context.Context, infra Outputs, localDir string, version string) error {
	prefix := infra.ArtifactsKey(version)

	objects, err := ListObjects(ctx, infra, prefix)
	if err != nil {
		return fmt.Errorf("error listing objects: %w", err)
	}

	s3Client := s3.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	logger := log.New(os.Stderr, "", 0)

	for _, obj := range objects {
		key := *obj.Key
		relPath := strings.TrimPrefix(key, prefix)
		if relPath == "" || strings.HasSuffix(relPath, "/") {
			continue
		}

		localPath := filepath.Join(localDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("error creating directory for %q: %w", localPath, err)
		}

		out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &infra.ArtifactsBucketName,
			Key:    &key,
		})
		if err != nil {
			return fmt.Errorf("error downloading %q: %w", key, err)
		}

		file, err := os.Create(localPath)
		if err != nil {
			out.Body.Close()
			return fmt.Errorf("error creating local file %q: %w", localPath, err)
		}
		_, err = io.Copy(file, out.Body)
		out.Body.Close()
		file.Close()
		if err != nil {
			return fmt.Errorf("error writing %q: %w", localPath, err)
		}
		logger.Println(fmt.Sprintf("Downloaded %s", key))
	}

	return nil
}
