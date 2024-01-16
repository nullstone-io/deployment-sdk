package lambda_container

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/lambda"
	nslambda "github.com/nullstone-io/deployment-sdk/aws/lambda"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"time"
)

func NewDeployer(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return Deployer{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Deployer struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	waitForChangesHeartbeat := func() {
		fmt.Fprintf(stdout, "Waiting for AWS to apply changes to lambda...\n")
	}

	fmt.Fprintf(stdout, "Deploying app %q\n", d.Details.App.Name)
	if meta.Version == "" {
		return "", fmt.Errorf("--version is required to deploy app")
	}

	// Update lambda function configuration (env vars)
	fmt.Fprintf(stdout, "Updating lambda environment variables\n")
	config, err := nslambda.GetFunctionConfig(ctx, d.Infra)
	if err != nil {
		return "", fmt.Errorf("error retrieving lambda configuration: %w", err)
	}
	updates := lambda.MapFunctionConfig(config)
	env_vars.UpdateStandard(updates.Environment.Variables, meta)
	if err := nslambda.UpdateFunctionConfig(ctx, d.Infra, updates); err != nil {
		return "", fmt.Errorf("error updating lambda configuration: %w", err)
	}
	// Wait for function configuration to take effect
	if err := nslambda.WaitForFunctionChanges(ctx, d.Infra, time.Minute, waitForChangesHeartbeat); err != nil {
		return "", fmt.Errorf("error waiting for updated lambda configuration: %w", err)
	}
	fmt.Fprintf(stdout, "Updated lambda environment variables\n")

	// Update lambda code version
	fmt.Fprintf(stdout, "Updating lambda code to %q\n", meta.Version)
	if err := UpdateLambdaVersion(ctx, d.Infra, meta.Version); err != nil {
		return "", fmt.Errorf("error updating lambda code version: %w", err)
	}
	// Wait for function code version to take effect
	if err := nslambda.WaitForFunctionChanges(ctx, d.Infra, time.Minute, waitForChangesHeartbeat); err != nil {
		return "", fmt.Errorf("error waiting for updated lambda configuration: %w", err)
	}
	fmt.Fprintf(stdout, "Updated lambda code\n")

	fmt.Fprintf(stdout, "Deployed app %q\n", d.Details.App.Name)
	return "", nil
}
