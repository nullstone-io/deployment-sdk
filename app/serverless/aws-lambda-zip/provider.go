package aws_lambda_zip

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/lambda-zip"
	"github.com/nullstone-io/deployment-sdk/aws/s3"
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
	NewPusher:             s3.NewZipPusher,
	NewDeployer:           lambda_zip.NewDeployer,
	NewDeployStatusGetter: nil,
}
