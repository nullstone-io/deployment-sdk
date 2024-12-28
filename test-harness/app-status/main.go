package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/gcp/gke"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/find"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"log"
)

func main() {
	stackId := int64(2643)
	blockId := int64(2911)
	envId := int64(571)

	ctx := context.Background()
	osWriters := logging.StandardOsWriters{}

	cfg := api.DefaultConfig()
	cfg.OrgName = "nullstone"
	rs := outputs.ApiRetrieverSource{Config: cfg}
	client := api.Client{Config: cfg}

	wd, err := client.WorkspaceDetails().Get(ctx, stackId, blockId, envId, false)
	if err != nil {
		log.Fatalln(err)
	}

	if wd.BlockType() != types.BlockTypeApplication {
		log.Fatalf("block is not an Application (actual = %s)\n", wd.BlockType())
	}
	application, _ := wd.Block().(types.Application)

	block := wd.AsBlock()
	appDetails := app.Details{
		App: &application,
		Env: &wd.Env,
	}
	appDetails.Module, err = find.Module(ctx, client.Config, appDetails.App.ModuleSource)
	if err != nil {
		log.Fatalln(err)
	}
	appDetails.Workspace, _ = client.Workspaces().Get(ctx, block.StackId, block.Id, wd.Env.Id)

	statuser, err := gke.NewStatuser(ctx, osWriters, rs, appDetails)
	if err != nil {
		log.Fatalln(err)
	}

	data, err := statuser.Status(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	raw, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(raw))
}
