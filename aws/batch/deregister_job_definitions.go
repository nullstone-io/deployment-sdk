package batch

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	batchtypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
)

func DeregisterJobDefinitions(ctx context.Context, infra Outputs, jobDefs []batchtypes.JobDefinition) ([]int32, error) {
	client := batch.NewFromConfig(nsaws.NewConfig(infra.Deployer, infra.Region))

	deregisteredRevisions := make([]int32, 0, len(jobDefs))
	for _, jobDef := range jobDefs {
		_, err := client.DeregisterJobDefinition(ctx, &batch.DeregisterJobDefinitionInput{
			JobDefinition: jobDef.JobDefinitionArn,
		})
		if err != nil {
			return deregisteredRevisions, fmt.Errorf("error deregistering job definition: %w", err)
		}
		deregisteredRevisions = append(deregisteredRevisions, *jobDef.Revision)
	}
	return deregisteredRevisions, nil
}
