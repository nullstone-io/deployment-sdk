package aws_s3

import (
	"github.com/nullstone-io/deployment-sdk/block"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppStaticSite),
	Provider:    "aws",
	Platform:    "s3",
	Subplatform: "",
}

var Provider = block.Provider{
	NewMetricsGetter: nil,
}
