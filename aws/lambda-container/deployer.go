package lambda_container

import (
	"context"
	"errors"
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
	fmt.Fprintf(stderr, "	region:         %s\n", d.Infra.Region)
	fmt.Fprintf(stderr, "	lambda_name:    %s\n", d.Infra.LambdaName)
	fmt.Fprintf(stderr, "	image_repo_url: %s\n", d.Infra.ImageRepoUrl)
}

func (d Deployer) Deploy(ctx context.Context, meta app.DeployMetadata) (string, error) {
	stderr := d.OsWriters.Stderr()
	d.Print()

	// heartbeat reports, while AWS is still applying a change, what we're waiting on and how long
	// it has taken so far. AWS does not surface progress, so the elapsed time reassures the user
	// that the deploy is still running (a container image update can take several minutes).
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
	colorstring.Fprintf(stderr, "[bold]Updating container image to version %q\n", meta.Version)
	imageUri, err := UpdateLambdaVersion(ctx, d.Infra, meta.Version)
	if err != nil {
		return "", fmt.Errorf("error updating lambda code version: %w", err)
	}
	fmt.Fprintf(stderr, "	image: %s\n", imageUri)
	// Wait for function code version to take effect
	if err := nslambda.WaitForFunctionChanges(ctx, d.Infra, 12*time.Minute, heartbeat("pull and apply the new container image")); err != nil {
		// A container image update normally finishes in well under a minute; a timeout here means
		// AWS is taking far longer than expected (large images can be slow for AWS to pull).
		if errors.Is(err, nslambda.ErrTimeoutWaitingForChanges) {
			return "", fmt.Errorf("AWS took much longer than expected (over 12 minutes) to apply the new container image. This usually resolves on a retry; large images can be slow for AWS to pull. Please retry the deployment.")
		}
		return "", fmt.Errorf("error waiting for updated lambda code: %w", err)
	}
	colorstring.Fprintln(stderr, "[green]Container image updated")

	colorstring.Fprintf(stderr, "[bold]Deployed app %q\n", d.Details.App.Name)
	return "", nil
}
