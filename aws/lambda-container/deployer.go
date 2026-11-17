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
	if d.Infra.LambdaAlias != "" {
		fmt.Fprintf(stderr, "	lambda_alias:   %s\n", d.Infra.LambdaAlias)
		keep, usingDefault := d.Infra.VersionsToKeep()
		if usingDefault {
			fmt.Fprintf(stderr, "	lambda_versions_to_keep: %d (using default)\n", keep)
		} else {
			fmt.Fprintf(stderr, "	lambda_versions_to_keep: %d\n", keep)
		}
	}
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

	// Without an alias, triggers invoke the function directly ($LATEST), so the deploy is already live.
	// With an alias, the deploy only takes effect once the alias points at a published version.
	if d.Infra.LambdaAlias != "" {
		if err := d.rolloutVersion(ctx, meta, heartbeat); err != nil {
			return "", err
		}
	}

	colorstring.Fprintf(stderr, "[bold]Deployed app %q\n", d.Details.App.Name)
	return "", nil
}

// versionActiveTimeout bounds how long we wait for AWS to prepare a published version.
// With SnapStart, AWS initializes the function and snapshots it before the version becomes usable.
const versionActiveTimeout = 10 * time.Minute

// rolloutVersion snapshots what was just deployed as a numbered version of the function, waits for AWS to
// prepare it, then moves the app's alias to it; this is the moment the deploy takes effect.
// The alias cannot move any earlier: AWS rejects pointing an alias at a version that is not ready.
// Older versions are then pruned.
func (d Deployer) rolloutVersion(ctx context.Context, meta app.DeployMetadata, heartbeat func(activity string) func(elapsed time.Duration)) error {
	stderr := d.OsWriters.Stderr()

	colorstring.Fprintln(stderr, "[bold]Publishing lambda version")
	version, err := nslambda.PublishVersion(ctx, d.Infra, fmt.Sprintf("app version %s", meta.Version))
	if err != nil {
		return fmt.Errorf("error publishing lambda version: %w", err)
	}
	fmt.Fprintf(stderr, "	lambda version: %s\n", version)
	if err := nslambda.WaitForVersionActive(ctx, d.Infra, version, versionActiveTimeout, heartbeat("prepare the published version")); err != nil {
		return fmt.Errorf("error waiting for lambda version %s: %w", version, err)
	}
	colorstring.Fprintln(stderr, "[green]Lambda version published")

	colorstring.Fprintf(stderr, "[bold]Pointing alias %q at lambda version %s\n", d.Infra.LambdaAlias, version)
	if err := nslambda.UpdateAlias(ctx, d.Infra, d.Infra.LambdaAlias, version); err != nil {
		return fmt.Errorf("error updating lambda alias %q: %w", d.Infra.LambdaAlias, err)
	}
	colorstring.Fprintln(stderr, "[green]Alias updated")

	// The deploy is live at this point; failing to clean up old versions should not fail it.
	keep, _ := d.Infra.VersionsToKeep()
	deleted, err := nslambda.PruneVersions(ctx, d.Infra, keep)
	if len(deleted) > 0 {
		fmt.Fprintf(stderr, "	removed %d old lambda version(s)\n", len(deleted))
	}
	if err != nil {
		colorstring.Fprintf(stderr, "[yellow]Could not remove old lambda versions: %s\n", err)
	}
	return nil
}
