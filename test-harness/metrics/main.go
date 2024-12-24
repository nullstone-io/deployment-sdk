package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/gcp/cloudmonitoring"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/find"
	"log"
	"time"
)

func main() {
	stackId := int64(2643)
	blockId := int64(2911)
	envId := int64(571)

	now := time.Now()
	start := now.Add(-time.Hour)
	options := workspace.MetricsGetterOptions{
		StartTime: &start,
		EndTime:   &now,
	}

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

	block := wd.AsBlock()
	appDetails := workspace.Details{
		Block: &block,
		Env:   &wd.Env,
	}
	appDetails.Module, err = find.Module(ctx, client.Config, appDetails.Block.ModuleSource)
	if err != nil {
		log.Fatalln(err)
	}
	appDetails.Workspace, _ = client.Workspaces().Get(ctx, block.StackId, block.Id, wd.Env.Id)

	getter, err := cloudmonitoring.NewGetter(ctx, osWriters, rs, appDetails)
	if err != nil {
		log.Fatalln(err)
	}

	data, err := getter.GetMetrics(ctx, options)
	if err != nil {
		log.Fatalln(err)
	}
	raw, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(raw))
}
