package cloudfunctions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

func NewDeployWatcher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployWatcher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return DeployWatcher{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

// DeployWatcher monitors the status of a Cloud Function deployment
type DeployWatcher struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (w DeployWatcher) Watch(ctx context.Context, reference string, isFirstDeploy bool) error {
	stdout, stderr := w.OsWriters.Stdout(), w.OsWriters.Stderr()

	if reference == "" {
		fmt.Fprintf(stdout, "This deployment does not have to wait for any resource to become healthy.\n")
		return nil
	}

	client, err := NewCloudFunctionsClient(ctx, w.Infra.Deployer)
	if err != nil {
		return fmt.Errorf("error creating Cloud Functions client: %w", err)
	}
	defer client.Close()

	fmt.Fprintf(stdout, "Waiting for Cloud Functions to build and deploy the cloud function...\n")

	delay := 5 * time.Second
	timeout := 15 * time.Minute

	t1 := time.After(timeout)
	for {
		// Check on the status of operation
		op, err := client.GetOperation(ctx, &longrunningpb.GetOperationRequest{Name: reference})
		if err != nil {
			return fmt.Errorf("error getting operation status: %w", err)
		}
		if op.Done {
			if operr := op.GetError(); operr != nil {
				fmt.Fprintf(stderr, "Deployment failed: %s\n", operr.Message)
				return app.ErrFailed
			}
			return nil
		}
		fmt.Fprintln(stdout, "Build/deploy in progress...")

		select {
		case <-ctx.Done():
			if cerr := ctx.Err(); cerr != nil {
				if errors.Is(cerr, context.DeadlineExceeded) {
					return app.ErrTimeout
				}
				return &app.CancelError{Reason: cerr.Error()}
			}
			return &app.CancelError{}
		case <-t1:
			// deployment timeout
			return app.ErrTimeout
		case <-time.After(delay):
			// poll for next update
			continue
		}
	}
}
