package aws_lambda_zip

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/lambda-zip"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServerless),
	Provider:    "aws",
	Platform:    "lambda",
	Subplatform: "zip",
}

var Provider = app.Provider{
	NewPusher:             lambda_zip.NewPusher,
	NewDeployer:           lambda_zip.NewDeployer,
	NewDeployStatusGetter: nil,
}
