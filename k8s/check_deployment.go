package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	appsv1 "k8s.io/api/apps/v1"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
)

// CheckDeployment maps the Deployment status to a friendly app.RolloutStatus
func CheckDeployment(deployment *appsv1.Deployment) (app.RolloutStatus, error) {
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := deploymentutil.GetDeploymentCondition(deployment.Status, appsv1.DeploymentProgressing)
		if cond != nil && cond.Reason == deploymentutil.TimedOutReason {
			return app.RolloutStatusFailed, fmt.Errorf("deployment failed because of timeout (exceeding its deadline)")
		}
		if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return app.RolloutStatusInProgress, nil
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return app.RolloutStatusInProgress, nil
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return app.RolloutStatusInProgress, nil
		}
		return app.RolloutStatusComplete, nil
	}
	return app.RolloutStatusPending, nil
}
