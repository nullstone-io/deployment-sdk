package aws_lambda_container

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/cloudwatch"
	"github.com/nullstone-io/deployment-sdk/aws/ecr"
	"github.com/nullstone-io/deployment-sdk/aws/lambda"
	"github.com/nullstone-io/deployment-sdk/aws/lambda-container"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServerless),
	Provider:    "aws",
	Platform:    "lambda",
	Subplatform: "container",
}

var Provider = app.Provider{
	CanDeployImmediate: true,
	NewPusher:          ecr.NewPusher,
	NewDeployer:        lambda_container.NewDeployer,
	NewDeployWatcher:   nil,
	NewStatuser:        nil,
	NewLogStreamer:     cloudwatch.NewLogStreamer,
	NewMetricsGetter:   lambda.NewMetricsGetter,
}
