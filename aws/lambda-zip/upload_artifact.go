package lambda_zip

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nullstone-io/deployment-sdk/aws"
	"io"
)

func UploadArtifact(ctx context.Context, infra Outputs, content io.ReadSeeker, version string) error {
	s3Client := s3.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	// Calculate md5 content to add as header (necessary for s3 buckets that have object lock enabled)
	// After calculating, we need to reset the content stream to transmit using s3.PutObject
	md5Summer := md5.New()
	if _, err := io.Copy(md5Summer, content); err != nil {
		return fmt.Errorf("error calculating md5 hash: %w", err)
	}
	md5Sum := base64.StdEncoding.EncodeToString(md5Summer.Sum(nil))
	if _, err := content.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("error resetting uploaded content after calculating md5 hash: %w", err)
	}

	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:     aws.String(infra.ArtifactsBucketName),
		Key:        aws.String(infra.ArtifactsKey(version)),
		Body:       content,
		ContentMD5: aws.String(md5Sum),
	})
	return err
}
