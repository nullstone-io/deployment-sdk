package aws_beanstalk

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/beanstalk"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServer),
	Provider:    "aws",
	Platform:    "ec2",
	Subplatform: "beanstalk",
}

var Provider = app.Provider{
	NewPusher:             beanstalk.NewPusher,
	NewDeployer:           beanstalk.NewDeployer,
	NewDeployStatusGetter: nil,
}
