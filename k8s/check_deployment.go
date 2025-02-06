package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	appsv1 "k8s.io/api/apps/v1"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"time"
)

// CheckDeployment maps the Deployment status to a friendly app.RolloutStatus
// This performs the same code as `kubectl rollout status` without revision checking
func CheckDeployment(deployment *appsv1.Deployment, expectedRevision int64) (*DeployEvent, app.RolloutStatus, error) {
	evt := &DeployEvent{Timestamp: time.Now(), Type: EventTypeNormal, Object: "Deployment"}

	latestRevision, err := Revision(deployment)
	if err != nil {
		return nil, app.RolloutStatusFailed, fmt.Errorf("unable to identify revision on the kubernetes deployment: %w", err)
	}

	if latestRevision < expectedRevision {
		// If the deployment has a revision smaller than the expected, it must not be in the k8s cluster yet
		evt.Message = "Waiting for deployment to start"
		return evt, app.RolloutStatusPending, nil
	} else if latestRevision > expectedRevision {
		// If the deployment has a revision larger than the expected, there must be a new deployment that invalidates this one
		evt.Type = EventTypeWarning
		evt.Message = fmt.Sprintf("A new deployment (revision = %d) was triggered which invalidates this deployment.", latestRevision)
		return evt, app.RolloutStatusFailed, fmt.Errorf(evt.Message)
	}

	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := deploymentutil.GetDeploymentCondition(deployment.Status, appsv1.DeploymentProgressing)
		if cond != nil && cond.Reason == deploymentutil.TimedOutReason {
			return nil, app.RolloutStatusFailed, fmt.Errorf("deployment failed because of timeout (exceeding its deadline)")
		}
		if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return nil, app.RolloutStatusInProgress, nil
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return nil, app.RolloutStatusInProgress, nil
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return nil, app.RolloutStatusInProgress, nil
		}
		return nil, app.RolloutStatusComplete, nil
	}
	return nil, app.RolloutStatusPending, nil
}
