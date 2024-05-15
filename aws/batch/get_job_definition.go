package batch

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	batchtypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func GetJobDefinition(ctx context.Context, infra Outputs) (*batchtypes.JobDefinition, []batchtypes.JobDefinition, error) {
	client := batch.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))
	active := "ACTIVE"
	lookup := &batch.DescribeJobDefinitionsInput{
		JobDefinitionName: &infra.JobDefinitionName,
		Status:            &active,
	}
	jobDefs, err := client.DescribeJobDefinitions(ctx, lookup)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting job definition: %w", err)
	}
	if len(jobDefs.JobDefinitions) == 0 {
		return nil, nil, fmt.Errorf("job definition named %q not found", infra.JobDefinitionName)
	}
	return getLatestJobDefinition(jobDefs.JobDefinitions), jobDefs.JobDefinitions, nil
}

func getLatestJobDefinition(defs []batchtypes.JobDefinition) *batchtypes.JobDefinition {
	var latest *batchtypes.JobDefinition
	for _, def := range defs {
		if latest == nil || (def.Revision != nil && *def.Revision > *latest.Revision) {
			newLatest := def
			latest = &newLatest
		}
	}
	return latest
}
