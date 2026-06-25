package composer

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// DeployWatcher monitors the long-running operation produced by a Composer environment update.
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

	client, err := NewEnvironmentsClient(ctx, w.Infra.Deployer)
	if err != nil {
		return fmt.Errorf("error creating Composer client: %w", err)
	}
	defer client.Close()

	fmt.Fprintf(stdout, "Waiting for the Composer environment to apply the updated configuration...\n")

	// Composer environment updates are slow (often 10-25 minutes), so we allow a generous timeout.
	delay := 15 * time.Second
	timeout := 30 * time.Minute

	op := client.UpdateEnvironmentOperation(reference)
	t1 := time.After(timeout)
	for {
		_, pollErr := op.Poll(ctx)
		if op.Done() {
			// When the operation finishes with an error, Poll reports it.
			if pollErr != nil {
				fmt.Fprintf(stderr, "Deployment failed: %s\n", pollErr)
				return app.ErrFailed
			}
			return nil
		}
		// Not done yet: a non-nil pollErr here is a transient polling error, so keep waiting.
		fmt.Fprintln(stdout, "Update in progress...")

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
			return app.ErrTimeout
		case <-time.After(delay):
			continue
		}
	}
}
