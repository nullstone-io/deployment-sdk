package app

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

// successful path test:
// 		the provider returns a couple statuses of in-progress followed by success
// 		want to ensure infra taskMessages are logged
// 		want to ensure nullstone progress is logged (waiting for healthy... (10s)
// provider.status is not implemented
// 		ensure that this is logged
// 		want the health check to immediately return healthy
// provider.status returns a status of failed
// 		make sure any taskMessages are logged
// 		make sure the result is logged
// 		ensure that we return unhealthy via an error message
// cancel
// 		calls the provider.CancelDeploy
// 		returns an error to indicate unhealthy
// timeout
// 		calls the provider.CancelDeploy
// 		returns an error to indicate unhealthy

func TestPollingDeployWatcher(t *testing.T) {
	type statusCall struct {
		rolloutStatus RolloutStatus
		error         error
	}
	tests := []struct {
		name        string
		statusCalls []statusCall
		expects     error
	}{
		{
			name: "success",
			statusCalls: []statusCall{
				{
					rolloutStatus: RolloutStatusInProgress,
				},
				{
					rolloutStatus: RolloutStatusComplete,
				},
			},
			expects: nil,
		},
		{
			name: "provider#status is not implemented",
			statusCalls: []statusCall{
				{
					error: fmt.Errorf("status is not supported for the ec2 provider"),
				},
				{
					error: fmt.Errorf("status is not supported for the ec2 provider"),
				},
			},
			expects: ErrTimeout,
		},
		{
			name: "provider#status returns a status of failed",
			statusCalls: []statusCall{
				{
					rolloutStatus: RolloutStatusFailed,
				},
			},
			expects: ErrFailed,
		},
		{
			name: "deployment times out",
			statusCalls: []statusCall{
				{
					rolloutStatus: RolloutStatusInProgress,
				},
				{
					rolloutStatus: RolloutStatusInProgress,
				},
			},
			expects: ErrTimeout,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			osWriters := logging.StandardOsWriters{}

			// setup our mock app.Provider for us to simulate `Status` calls
			// the responses from app.Provider#status are what controls how WaitHealthy behaves
			mockGetter := &MockDeployStatusGetter{}
			mockGetter.Test(t)
			mockWatcher := &PollingDeployWatcher{
				StatusGetter: mockGetter,
				OsWriters:    osWriters,
				Delay:        1 * time.Millisecond,
				Timeout:      2 * time.Millisecond,
			}

			for _, call := range test.statusCalls {
				mockGetter.On("GetDeployStatus",
					mock.Anything,
					mock.AnythingOfType("string")).
					Return(call.rolloutStatus, call.error).
					Once()
			}
			mockGetter.On("Close")

			err := mockWatcher.Watch(ctx, "stub", false)
			mockGetter.AssertExpectations(t)

			if test.expects == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expects.Error())
			}
		})
	}
}
