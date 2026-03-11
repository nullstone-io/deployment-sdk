package azure_aca

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/azure/aca"
	"github.com/nullstone-io/deployment-sdk/azure/acr"
	"github.com/nullstone-io/deployment-sdk/azure/azuremonitor"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "azure",
	Platform:    "aca",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: false,
	NewPusher:          acr.NewPusher,
	NewDeployer:        aca.NewDeployer,
	NewDeployWatcher:   aca.NewDeployWatcher,
	NewLogStreamer:     azuremonitor.NewLogStreamer,
}
