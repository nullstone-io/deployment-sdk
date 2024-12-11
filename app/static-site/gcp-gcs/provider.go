package gcp_gcs

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/cloudcdn"
	"github.com/nullstone-io/deployment-sdk/gcp/gcs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppStaticSite),
	Provider:    "gcp",
	Platform:    "gcs",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: true,
	NewPusher:          gcs.NewDirPusher,
	NewDeployer:        gcs.NewDeployer,
	NewDeployWatcher:   app.NewPollingDeployWatcher(cloudcdn.NewDeployStatusGetter),
	NewStatuser:        nil,
	NewLogStreamer:     nil, //TODO: Implement cloudlogging.NewLogStreamer,
}
