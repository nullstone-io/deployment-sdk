package aws_ecs_fargate

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch/metrics"
	"github.com/nullstone-io/deployment-sdk/block"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "aws",
	Platform:    "ecs",
	Subplatform: "fargate",
}

var Provider = block.Provider{
	NewMetricsGetter: metrics.NewGetter,
}
