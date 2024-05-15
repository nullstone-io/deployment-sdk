package batch

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	batchtypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func CreateJobDefinition(ctx context.Context, infra Outputs, jobDefinition *batchtypes.JobDefinition) (*string, *int32, error) {
	client := batch.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	input := &batch.RegisterJobDefinitionInput{
		JobDefinitionName:    jobDefinition.JobDefinitionName,
		Type:                 batchtypes.JobDefinitionType(aws.ToString(jobDefinition.Type)),
		ContainerProperties:  jobDefinition.ContainerProperties,
		EcsProperties:        jobDefinition.EcsProperties,
		EksProperties:        jobDefinition.EksProperties,
		NodeProperties:       jobDefinition.NodeProperties,
		Parameters:           jobDefinition.Parameters,
		PlatformCapabilities: jobDefinition.PlatformCapabilities,
		PropagateTags:        jobDefinition.PropagateTags,
		RetryStrategy:        jobDefinition.RetryStrategy,
		SchedulingPriority:   jobDefinition.SchedulingPriority,
		Tags:                 jobDefinition.Tags,
		Timeout:              jobDefinition.Timeout,
	}
	out, err := client.RegisterJobDefinition(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to register job definition: %w", err)
	}

	return out.JobDefinitionArn, out.Revision, nil
}
