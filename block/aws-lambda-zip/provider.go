package aws_lambda_zip

import (
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch/metrics"
	"github.com/nullstone-io/deployment-sdk/block"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServerless),
	Provider:    "aws",
	Platform:    "lambda",
	Subplatform: "zip",
}

var Provider = block.Provider{
	NewMetricsGetter: metrics.NewGetter,
}
