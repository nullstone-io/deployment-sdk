package lambda_zip

import (
	"context"
	"fmt"
	"time"

	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws/lambda"
	nslambda "github.com/nullstone-io/deployment-sdk/aws/lambda"
	env_vars "github.com/nullstone-io/deployment-sdk/env-vars"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Deployer, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

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

func (d Deployer) Print() {
	stderr := d.OsWriters.Stderr()
	colorstring.Fprintln(stderr, "[bold]Retrieved Lambda outputs")
	fmt.Fprintf(stderr, "	region:                %s\n", d.Infra.Region)
	fmt.Fprintf(stderr, "	lambda_name:           %s\n", d.Infra.LambdaName)
	fmt.Fprintf(stderr, "	artifacts_bucket_name: %s\n", d.Infra.ArtifactsBucketName)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stderr := d.OsWriters.Stderr()
	d.Print()

	// heartbeat reports, while AWS is still applying a change, what we're waiting on and how long
	// it has taken so far. AWS does not surface progress, so the elapsed time reassures the user
	// that the deploy is still running.
	heartbeat := func(activity string) func(elapsed time.Duration) {
		return func(elapsed time.Duration) {
			fmt.Fprintf(stderr, "	Waiting for AWS to %s (%s elapsed)\n", activity, elapsed)
		}
	}

	fmt.Fprintln(stderr)
	colorstring.Fprintf(stderr, "[bold]Deploying app %q\n", d.Details.App.Name)
	if meta.Version == "" {
		return "", fmt.Errorf("--version is required to deploy app")
	}

	// Update lambda function configuration (env vars)
	colorstring.Fprintln(stderr, "[bold]Updating environment variables")
	config, err := nslambda.GetFunctionConfig(ctx, d.Infra)
	if err != nil {
		return "", fmt.Errorf("error retrieving lambda configuration: %w", err)
	}
	updates := lambda.MapFunctionConfig(config)
	env_vars.UpdateStandard(updates.Environment.Variables, meta)
	if updated, changed := env_vars.ReplaceOtelResourceAttributes(updates.Environment.Variables, meta, false); changed {
		updates.Environment.Variables = updated
		fmt.Fprintln(stderr, "	updating OpenTelemetry resource attributes (service.version and service.commit.sha)")
	}
	if err := nslambda.UpdateFunctionConfig(ctx, d.Infra, updates); err != nil {
		return "", fmt.Errorf("error updating lambda configuration: %w", err)
	}
	// Wait for function configuration to take effect
	if err := nslambda.WaitForFunctionChanges(ctx, d.Infra, time.Minute, heartbeat("apply configuration changes")); err != nil {
		return "", fmt.Errorf("error waiting for updated lambda configuration: %w", err)
	}
	colorstring.Fprintln(stderr, "[green]Environment variables updated")

	// Update lambda code version
	colorstring.Fprintf(stderr, "[bold]Updating code to version %q\n", meta.Version)
	if err := UpdateLambdaVersion(ctx, d.Infra, meta.Version); err != nil {
		return "", fmt.Errorf("error updating lambda version: %w", err)
	}
	// Wait for function code version to take effect
	if err := nslambda.WaitForFunctionChanges(ctx, d.Infra, time.Minute, heartbeat("apply the new code")); err != nil {
		return "", fmt.Errorf("error waiting for updated lambda code: %w", err)
	}
	colorstring.Fprintln(stderr, "[green]Code updated")

	colorstring.Fprintf(stderr, "[bold]Deployed app %q\n", d.Details.App.Name)
	return "", nil
}
