package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func PutEnvVars(ctx context.Context, infra Outputs, envVars map[string]string) error {
	raw, _ := json.Marshal(envVars)

	s3Client := s3.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(infra.ArtifactsBucketName),
		Key:         aws.String(infra.EnvVarsFilename),
		Body:        bytes.NewBuffer(raw),
		ContentType: aws.String("application/json"),
	})
	return err
}
