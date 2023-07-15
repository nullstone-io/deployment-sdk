package aws_s3

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/cdn"
	"github.com/nullstone-io/deployment-sdk/aws/s3"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppStaticSite),
	Provider:    "aws",
	Platform:    "s3",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: true,
	NewPusher:          s3.NewDirPusher,
	NewDeployer:        s3.NewDeployer,
	NewDeployWatcher:   app.NewPollingDeployWatcher(cdn.NewDeployStatusGetter),
	NewStatuser:        nil,
}
