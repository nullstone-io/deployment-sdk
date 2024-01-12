package aws_beanstalk

import (
	"github.com/nullstone-io/deployment-sdk/block"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServer),
	Provider:    "aws",
	Platform:    "ec2",
	Subplatform: "beanstalk",
}

var Provider = block.Provider{
	NewMetricsGetter: nil,
}
