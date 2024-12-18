package gcp_gke_service

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/gar"
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
	NewPusher:          gar.NewPusher,
	NewDeployer:        gke.NewDeployer,
	NewDeployWatcher:   gke.NewDeployWatcher,
	NewStatuser:        nil,
	NewLogStreamer:     gke.NewLogStreamer,
}
