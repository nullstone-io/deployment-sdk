package azure_functions

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/azure/azuremonitor"
	"github.com/nullstone-io/deployment-sdk/azure/functions"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServerless),
	Provider:    "azure",
	Platform:    "functions",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: true,
	NewDeployer:        functions.NewDeployer,
	NewDeployWatcher:   functions.NewDeployWatcher,
	NewLogStreamer:     azuremonitor.NewLogStreamer,
}
