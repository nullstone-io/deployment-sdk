package aws_lambda_container

import (
	"github.com/nullstone-io/deployment-sdk/aws/lambda"
	"github.com/nullstone-io/deployment-sdk/block"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServerless),
	Provider:    "aws",
	Platform:    "lambda",
	Subplatform: "container",
}

var Provider = block.Provider{
	NewMetricsGetter: lambda.NewMetricsGetter,
}
