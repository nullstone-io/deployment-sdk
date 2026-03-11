package azure_blob

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/azure/blob"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppStaticSite),
	Provider:    "azure",
	Platform:    "blob",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: true,
	NewPusher:          blob.NewDirPusher,
	NewDeployer:        blob.NewDeployer,
	NewDeployWatcher:   app.NewPollingDeployWatcher(blob.NewDeployStatusGetter),
}
