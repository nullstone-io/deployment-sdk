package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func GetTaskDefinitionTags(ctx context.Context, infra Outputs) ([]ecstypes.Tag, error) {
	ecsClient := ecs.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	out2, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
		ResourceArn: aws.String(infra.TaskArn),
	})
	if err != nil {
		return nil, err
	}
	return out2.Tags, nil
}
