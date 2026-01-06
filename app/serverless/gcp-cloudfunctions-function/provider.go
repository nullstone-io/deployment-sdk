package gcp_cloudfunctions_function

import (
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/cloudfunctions"
	"github.com/nullstone-io/deployment-sdk/gcp/cloudlogging"
	"github.com/nullstone-io/deployment-sdk/gcp/gcs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppServerless),
	Provider:    "gcp",
	Platform:    "cloudfunctions",
	Subplatform: "",
}

var Provider = app.Provider{
	CanDeployImmediate: true,
	NewPusher:          gcs.NewZipPusher,
	NewDeployer:        cloudfunctions.NewDeployer,
	NewDeployWatcher:   cloudfunctions.NewDeployWatcher,
	NewStatuser:        nil,
	NewLogStreamer:     cloudlogging.NewLogStreamer,
}
