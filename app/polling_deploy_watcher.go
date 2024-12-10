package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"time"
)

const (
	watchDefaultTimeout = 15 * time.Minute
	watchDefaultDelay   = 5 * time.Second
)

var (
	ErrTimeout = errors.New("deployment timed out")
	ErrFailed  = errors.New("deployment failed")
)

var _ DeployWatcher = &PollingDeployWatcher{}

// PollingDeployWatcher watches a deployment using polling
// The implementation supports cancellation and timeouts through a context.Context
type PollingDeployWatcher struct {
	StatusGetter DeployStatusGetter
	OsWriters    logging.OsWriters
	Delay        time.Duration
	Timeout      time.Duration
}

// NewPollingDeployWatcher wraps a DeployStatusGetter to provide polling support for watching a deployment
func NewPollingDeployWatcher(statusGetterFn NewDeployStatusGetterFunc) NewDeployWatcherFunc {
	return func(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (DeployWatcher, error) {
		statusGetter, err := statusGetterFn(ctx, osWriters, source, appDetails)
		if err != nil {
			return nil, err
		}
		return &PollingDeployWatcher{
			StatusGetter: statusGetter,
			OsWriters:    osWriters,
		}, nil
	}
}

// Watch polls the provider for rollout status on the deployment.
// This is long-running and supports cancellation/timeout via ctx
// This polls every 5s and times out after 15m
// This function has the following return values:
// - nil: deployment completed successfully
// - ErrFailed: Deployment failed as reported by DeployStatusGetter.GetDeployStatus
// - CancelError: System cancelled by evicted deployment or via ctx
// - ErrTimeout: ctx reached timeout or watcher reached 15m timeout
func (s *PollingDeployWatcher) Watch(ctx context.Context, reference string) error {
	stdout := s.OsWriters.Stdout()
	defer s.StatusGetter.Close()

	if reference == "" {
		fmt.Fprintf(stdout, "This deployment does not have to wait for any resource to become healthy.\n")
		return nil
	}

	delay, timeout := watchDefaultDelay, watchDefaultTimeout
	if s.Delay != 0 {
		delay = s.Delay
	}
	if s.Timeout != 0 {
		timeout = s.Timeout
	}
	t1 := time.After(timeout)
	for {
		status, err := s.StatusGetter.GetDeployStatus(ctx, reference)
		if err != nil {
			if status == RolloutStatusCancelled {
				return &CancelError{Reason: err.Error()}
			}
			// TODO: Differentiate between initialization and failure errors
			// -> initialization errors will keep polling (e.g. we don't want to stop polling if the service is booting)
			// -> failure errors will immediately fail the deployment (e.g. the deployment was cancelled/evicted)

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
			if err := ctx.Err(); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return ErrTimeout
				}
				return &CancelError{Reason: err.Error()}
			}
			return &CancelError{}
		case <-t1:
			return ErrTimeout
		case <-time.After(delay):
			// Poll status again
			continue
		}
	}
}
