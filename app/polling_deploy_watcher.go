package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/logging"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"time"
)

const (
	watchDefaultTimeout = 15 * time.Minute
	watchDefaultDelay   = 5 * time.Second
)

var (
	ErrTimeout   = errors.New("deployment timed out")
	ErrFailed    = errors.New("deployment failed")
	ErrCancelled = errors.New("deployment cancelled")
)

var _ DeployWatcher = &PollingDeployWatcher{}

// PollingDeployWatcher watches a deployment using polling
// The implementation supports cancellation and timeouts through a context.Context
type PollingDeployWatcher struct {
	StatusGetter DeployStatusGetter
	OsWriters    logging.OsWriters
	Details      Details
}

// NewPollingDeployWatcher wraps a DeployStatusGetter to provide polling support for watching a deployment
func NewPollingDeployWatcher(statusGetterFn NewDeployStatusGetterFunc) NewDeployWatcherFunc {
	return func(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (DeployWatcher, error) {
		statusGetter, err := statusGetterFn(osWriters, nsConfig, appDetails)
		if err != nil {
			return nil, err
		}
		return &PollingDeployWatcher{
			StatusGetter: statusGetter,
			OsWriters:    osWriters,
			Details:      appDetails,
		}, nil
	}
}

// Watch polls the provider for rollout status on the deployment.
// This is long-running and supports cancellation/timeout via ctx
// This polls every 5s and times out after 15m
// This function has the following return values:
// - nil: deployment completed successfully
// - ErrFailed: Deployment failed as reported by DeployStatusGetter.GetDeployStatus
// - ErrCancelled: System cancelled via ctx
// - ErrTimeout: ctx reached timeout or watcher reached 15m timeout
func (s *PollingDeployWatcher) Watch(ctx context.Context, reference string) error {
	stdout := s.OsWriters.Stdout()

	if reference == "" {
		fmt.Fprintf(stdout, "This deployment does not have to wait for any resource to become healthy.\n")
		return nil
	}

	delay, timeout := watchDefaultDelay, watchDefaultTimeout
	t1 := time.After(timeout)
	for {
		status, err := s.StatusGetter.GetDeployStatus(ctx, reference)
		if err != nil {
			// if for some reason we can't fetch the app status from the provider
			// we are going to log the error and continue looping
			// eventually the deploy will timeout and fail
			fmt.Fprintf(stdout, "error occurred fetching the deployment status from the provider: %s\n", err)
		} else {
			if status == RolloutStatusFailed {
				return ErrFailed
			}
			if status == RolloutStatusComplete {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return ErrTimeout
			}
			return ErrCancelled
		case <-t1:
			return ErrTimeout
		case <-time.After(delay):
			// Poll status again
			continue
		}
	}
}
