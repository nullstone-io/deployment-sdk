package s3

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

// ListDirs queries s3 buckets for a listing of keys to determine root directories
// S3 doesn't have the concept of directories;
// this simulates by finding a common set of prefixes preceding a `/` in the object key
func ListDirs(ctx context.Context, infra Outputs) ([]string, error) {
	s3Client := s3.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	visited := map[string]bool{}
	result := make([]string, 0)

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(infra.ArtifactsBucketName),
		Delimiter: aws.String("/"),
	}
	paginator := s3.NewListObjectsV2Paginator(s3Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing bucket objects: %w", err)
		}

		for _, prefix := range page.CommonPrefixes {
			dir := *prefix.Prefix
			if _, ok := visited[dir]; !ok {
				visited[dir] = true
				result = append(result, dir)
			}
		}
	}

	return result, nil
}
