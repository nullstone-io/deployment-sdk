package batch

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	batchtypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func GetJobDefinition(ctx context.Context, infra Outputs) (*batchtypes.JobDefinition, error) {
	client := batch.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	lookup := []string{infra.JobDefinitionArn}
	jobDefs, err := client.DescribeJobDefinitions(ctx, &batch.DescribeJobDefinitionsInput{JobDefinitions: lookup})
	if err != nil {
		return nil, fmt.Errorf("error getting job definition: %w", err)
	}
	if len(jobDefs.JobDefinitions) == 0 {
		return nil, fmt.Errorf("job definition named %q not found", infra.JobDefinitionName)
	}
	if len(jobDefs.JobDefinitions) > 1 {
		return nil, fmt.Errorf("found multiple job definitions with name %q", infra.JobDefinitionName)
	}
	jobDef := jobDefs.JobDefinitions[0]
	return &jobDef, nil
}
