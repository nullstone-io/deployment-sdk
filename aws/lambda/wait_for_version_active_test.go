package lambda

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateVersionState(t *testing.T) {
	tests := []struct {
		name             string
		state            types.State
		lastUpdateStatus types.LastUpdateStatus
		want             versionReadiness
	}{
		{name: "pending", state: types.StatePending, want: versionPending},
		{name: "failed", state: types.StateFailed, want: versionFailed},
		{name: "active", state: types.StateActive, want: versionReady},
		{name: "inactive", state: types.StateInactive, want: versionReady},
		{name: "not reported", want: versionReady},
		{name: "active and successful", state: types.StateActive, lastUpdateStatus: types.LastUpdateStatusSuccessful, want: versionReady},
		{name: "active but update in progress", state: types.StateActive, lastUpdateStatus: types.LastUpdateStatusInProgress, want: versionPending},
		{name: "active but update failed", state: types.StateActive, lastUpdateStatus: types.LastUpdateStatusFailed, want: versionFailed},
		{name: "pending but update failed", state: types.StatePending, lastUpdateStatus: types.LastUpdateStatusFailed, want: versionFailed},
		{name: "failed while update in progress", state: types.StateFailed, lastUpdateStatus: types.LastUpdateStatusInProgress, want: versionFailed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, evaluateVersionState(test.state, test.lastUpdateStatus))
		})
	}
}
