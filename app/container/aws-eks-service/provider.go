package aws_eks_service

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/ecr"
	"github.com/nullstone-io/deployment-sdk/aws/eks"
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
	NewDeployer:        eks.NewDeployer,
	NewDeployWatcher:   eks.NewDeployWatcher,
	NewStatuser:        nil,
	NewLogStreamer:     eks.NewLogStreamer,
}
