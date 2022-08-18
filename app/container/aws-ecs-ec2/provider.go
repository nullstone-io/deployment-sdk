package aws_ecs_ec2

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/ecr"
	"github.com/nullstone-io/deployment-sdk/aws/ecs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "aws",
	Platform:    "ecs",
	Subplatform: "ec2",
}

var Provider = app.Provider{
	CanDeployImmediate:    false,
	NewPusher:             ecr.NewPusher,
	NewDeployer:           ecs.NewDeployer,
	NewDeployStatusGetter: ecs.NewDeployStatusGetter,
}
