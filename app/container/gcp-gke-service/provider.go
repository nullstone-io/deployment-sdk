package gcp_gke_service

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/gcr"
	"github.com/nullstone-io/deployment-sdk/gcp/gke"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "gcp",
	Platform:    "k8s",
	Subplatform: "gke",
}

var Provider = app.Provider{
	CanDeployImmediate: false,
	NewPusher:          gcr.NewPusher,
	NewDeployer:        gke.NewDeployer,
	NewDeployWatcher:   app.NewPollingDeployWatcher(gke.NewDeployStatusGetter),
}
