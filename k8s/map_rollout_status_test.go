package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	"testing"
)

func TestMapRolloutStatus(t *testing.T) {
	tests := []struct {
		name       string
		deployment v1.Deployment
		want       app.RolloutStatus
		err        error
	}{
		{
			name: "timed out",
			deployment: v1.Deployment{
				Spec: v1.DeploymentSpec{},
				Status: v1.DeploymentStatus{
					Conditions: []v1.DeploymentCondition{
						{
							Type:   v1.DeploymentAvailable,
							Reason: "SomethingElse",
						},
						{
							Type:   v1.DeploymentProgressing,
							Reason: TimedOutReason,
						},
					},
				},
			},
			want: app.RolloutStatusFailed,
			err:  fmt.Errorf("deployment failed because timed out (exceeding its deadline)"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := MapRolloutStatus(test.deployment)
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.want, got)
		})
	}
}
