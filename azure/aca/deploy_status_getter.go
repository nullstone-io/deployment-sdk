package aca

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers/v2"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployStatusGetter(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return &DeployStatusGetter{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type DeployStatusGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d *DeployStatusGetter) Close() {}

func (d *DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	stdout := d.OsWriters.Stdout()

	if d.Infra.ContainerAppName == "" && d.Infra.JobName == "" {
		fmt.Fprintln(stdout, "No container app or job name in app module. Skipping check for healthy.")
		return app.RolloutStatusComplete, nil
	}

	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	// For jobs, we consider the update complete as soon as the deploy returns
	if d.Infra.JobName != "" && d.Infra.ContainerAppName == "" {
		fmt.Fprintln(stdout, "Job update completed.")
		return app.RolloutStatusComplete, nil
	}

	// For container apps, poll the revision status
	clientFactory, err := armappcontainers.NewClientFactory(d.Infra.SubscriptionId, &d.Infra.Deployer, nil)
	if err != nil {
		return app.RolloutStatusUnknown, fmt.Errorf("error creating ACA client: %w", err)
	}
	revisionsClient := clientFactory.NewContainerAppsRevisionsClient()

	revision, err := revisionsClient.GetRevision(ctx, d.Infra.ResourceGroup, d.Infra.ContainerAppName, reference, nil)
	if err != nil {
		return app.RolloutStatusUnknown, fmt.Errorf("error retrieving revision %q: %w", reference, err)
	}

	if revision.Properties == nil || revision.Properties.ProvisioningState == nil {
		fmt.Fprintln(stdout, "Waiting for revision provisioning state...")
		return app.RolloutStatusInProgress, nil
	}

	state := *revision.Properties.ProvisioningState
	switch state {
	case armappcontainers.RevisionProvisioningStateProvisioned:
		if revision.Properties.RunningState != nil {
			runState := *revision.Properties.RunningState
			switch runState {
			case armappcontainers.RevisionRunningStateRunning:
				fmt.Fprintln(stdout, "Revision is running.")
				return app.RolloutStatusComplete, nil
			case armappcontainers.RevisionRunningStateDegraded:
				fmt.Fprintln(stdout, "Revision is degraded.")
				return app.RolloutStatusFailed, nil
			case armappcontainers.RevisionRunningStateFailed:
				fmt.Fprintln(stdout, "Revision failed.")
				return app.RolloutStatusFailed, nil
			case armappcontainers.RevisionRunningStateStopped:
				fmt.Fprintln(stdout, "Revision stopped.")
				return app.RolloutStatusComplete, nil
			default:
				fmt.Fprintf(stdout, "Revision running state: %s\n", runState)
				return app.RolloutStatusInProgress, nil
			}
		}
		fmt.Fprintln(stdout, "Revision provisioned, waiting for running state...")
		return app.RolloutStatusInProgress, nil
	case armappcontainers.RevisionProvisioningStateProvisioning:
		fmt.Fprintln(stdout, "Revision is provisioning...")
		return app.RolloutStatusInProgress, nil
	case armappcontainers.RevisionProvisioningStateFailed:
		fmt.Fprintln(stdout, "Revision provisioning failed.")
		return app.RolloutStatusFailed, nil
	case armappcontainers.RevisionProvisioningStateDeprovisioning:
		fmt.Fprintln(stdout, "Revision is deprovisioning...")
		return app.RolloutStatusFailed, nil
	default:
		fmt.Fprintf(stdout, "Unknown provisioning state: %s\n", state)
		return app.RolloutStatusInProgress, nil
	}
}
