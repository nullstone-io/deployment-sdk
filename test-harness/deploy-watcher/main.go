package main

// This utility allows for direct debugging of the deploy watcher
//

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/gke"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"log"
)

func main() {
	stackId := int64(6)
	blockId := int64(53)
	envId := int64(30)
	reference := "6"

	ctx := context.Background()
	osWriters := logging.StandardOsWriters{}

	cfg := api.DefaultConfig()
	cfg.OrgName = "BSick7"
	rs := outputs.ApiRetrieverSource{Config: cfg}
	client := api.Client{Config: cfg}

	wd, err := client.WorkspaceDetails().Get(ctx, stackId, blockId, envId, false)
	if err != nil {
		log.Fatalln(err)
	}

	workspace, err := client.Workspaces().Get(ctx, stackId, blockId, envId)
	if err != nil {
		log.Fatalln(err)
	} else if workspace == nil {
		log.Fatalln("workspace does not exist")
	}
	wc, err := client.WorkspaceConfigs().GetCurrent(ctx, stackId, blockId, envId)
	if err != nil {
		log.Fatalln(err)
	}
	module, err := client.Modules().Get(ctx, wc.Source, wc.SourceVersion)
	if err != nil {
		log.Fatalln(err)
	}

	appl, _ := wd.Block().(types.Application)
	appDetails := app.Details{
		App:             &appl,
		Env:             &wd.Env,
		Workspace:       workspace,
		WorkspaceConfig: wc,
		Module:          module,
	}

	dw, err := gke.NewDeployWatcher(ctx, osWriters, rs, appDetails)
	if err := dw.Watch(ctx, reference); err != nil {
		log.Fatalln(err)
	}
}
