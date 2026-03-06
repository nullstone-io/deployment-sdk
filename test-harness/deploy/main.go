package main

// This utility allows for direct debugging of the deployer
//

import (
	"context"
	"log"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/gcs"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

func main() {
	stackId := int64(1)
	blockId := int64(2)
	envId := int64(3)

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

	d, err := gcs.NewDeployer(ctx, osWriters, rs, appDetails)
	if err != nil {
		log.Fatalln(err)
	}
	reference, err := d.Deploy(ctx, app.DeployMetadata{
		Version:     "608661e",
		CommitSha:   "608661e0d0c0cd4f5750fe27e0c2de0d0471d861",
		Type:        "",
		PackageMode: "",
	})
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(reference)
}
