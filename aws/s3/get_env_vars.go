package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func GetEnvVars(ctx context.Context, infra Outputs) (map[string]string, error) {
	s3Client := s3.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(infra.ArtifactsBucketName),
		Key:    aws.String(infra.EnvVarsFilename),
	})
	if err != nil {
		return nil, fmt.Errorf("error retrieving existing environment variables file: %w", err)
	}

	defer out.Body.Close()
	decoder := json.NewDecoder(out.Body)
	m := map[string]string{}
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("environment variables file is an invalid format: %w", err)
	}
	return m, nil
}
