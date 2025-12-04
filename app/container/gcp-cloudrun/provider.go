package gcp_cloudrun

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/cloudrun"
	"github.com/nullstone-io/deployment-sdk/gcp/gar"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "gcp",
	Platform:    "cloudrun",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: false,
	NewPusher:          gar.NewPusher,
	NewDeployer:        cloudrun.NewDeployer,
	NewDeployWatcher:   cloudrun.NewDeployWatcher,
	//NewStatuser:        cloudrun.NewStatuser,
	//NewLogStreamer:     cloudrun.NewLogStreamer,
}
