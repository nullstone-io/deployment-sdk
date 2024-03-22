package aws_batch_fargate

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/batch"
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/aws/ecr"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "aws",
	Platform:    "batch",
	Subplatform: "fargate",
}

var Provider = app.Provider{
	CanDeployImmediate: false,
	NewPusher:          ecr.NewPusher,
	NewDeployer:        batch.NewDeployer,
	NewLogStreamer:     cloudwatch.NewLogStreamer,
}
