package gcp_composer

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/cloudlogging"
	"github.com/nullstone-io/deployment-sdk/gcp/composer"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServerless),
	Provider:    "gcp",
	Platform:    "composer",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: true,
	NewPusher:          composer.NewPusher,
	NewDeployer:        composer.NewDeployer,
	NewDeployWatcher:   composer.NewDeployWatcher,
	NewStatuser:        nil,
	NewLogStreamer:     cloudlogging.NewLogStreamer,
}
