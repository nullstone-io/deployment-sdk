package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	v1 "k8s.io/api/apps/v1"
)

const (
	TimedOutReason = "ProgressDeadlineExceeded"
)

func MapRolloutStatus(deployment v1.Deployment) (app.RolloutStatus, error) {
	status := deployment.Status
	spec := deployment.Spec

	for _, cond := range status.Conditions {
		if cond.Type == v1.DeploymentProgressing {
			// If we found a "DeploymentProgressing", check to see if we timed out
			if cond.Reason == TimedOutReason {
				return app.RolloutStatusFailed, fmt.Errorf("deployment failed because timed out (exceeding its deadline)")
			}
			break
		}
	}

	if spec.Replicas != nil && status.UpdatedReplicas < *spec.Replicas {
		return app.RolloutStatusInProgress, nil
	}
	if status.Replicas > status.UpdatedReplicas {
		return app.RolloutStatusInProgress, nil
	}
	if status.AvailableReplicas < status.UpdatedReplicas {
		return app.RolloutStatusInProgress, nil
	}
	return app.RolloutStatusComplete, nil
}
