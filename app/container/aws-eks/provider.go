package aws_eks

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/ecr"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "aws",
	Platform:    "k8s",
	Subplatform: "eks",
}

var Provider = app.Provider{
	CanDeployImmediate: false,
	NewPusher:          ecr.NewPusher,
	NewDeployer:        nil,
	NewDeployWatcher:   nil,
	NewStatuser:        nil,
}
