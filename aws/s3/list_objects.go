package s3

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func ListObjects(ctx context.Context, infra Outputs, prefix string) ([]types.Object, error) {
	s3Client := s3.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	result := make([]types.Object, 0)

	input := &s3.ListObjectsV2Input{Bucket: aws.String(infra.ArtifactsBucketName)}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	paginator := s3.NewListObjectsV2Paginator(s3Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing bucket objects: %w", err)
		}

		for _, object := range page.Contents {
			result = append(result, object)
		}
	}

	return result, nil
}
