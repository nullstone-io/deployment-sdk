package aws_ec2

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServer),
	Provider:    "aws",
	Platform:    "ec2",
	Subplatform: "",
}

var Provider = app.Provider{
	NewPusher:             nil,
	NewDeployer:           nil,
	NewDeployStatusGetter: nil,
}
