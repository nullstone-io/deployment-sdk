package s3

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func DownloadZipArtifact(ctx context.Context, infra Outputs, localPath string, version string) error {
	s3Client := s3.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	key := infra.ArtifactsKey(version)
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &infra.ArtifactsBucketName,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("error downloading %q: %w", key, err)
	}
	defer out.Body.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("error creating local file %q: %w", localPath, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, out.Body); err != nil {
		return fmt.Errorf("error writing %q: %w", localPath, err)
	}

	return nil
}
