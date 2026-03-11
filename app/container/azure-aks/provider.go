package azure_aks

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/azure/acr"
	"github.com/nullstone-io/deployment-sdk/azure/aks"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "azure",
	Platform:    "k8s",
	Subplatform: "aks",
}

var Provider = app.Provider{
	CanDeployImmediate: false,
	NewPusher:          acr.NewPusher,
	NewDeployer:        aks.NewDeployer,
	NewDeployWatcher:   aks.NewDeployWatcher,
}
