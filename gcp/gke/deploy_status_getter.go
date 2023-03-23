package gke

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	v1 "k8s.io/api/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"sync"
)

func NewDeployStatusGetter(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return &DeployStatusGetter{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type DeployStatusGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs

	startDeployment sync.Once
	numDesired      int
}

func (d *DeployStatusGetter) initialize(ctx context.Context, reference string) error {
	return nil
}

// GetDeployStatus resolves the current status of the gke deployment
// A Kubernetes Deployment allows for declarative updates for Pods and ReplicaSets
// A Deployment is a desired state and is not versioned
// However, a Deployment has a revision which we will track
// Note: Scaling deployments does not trigger rollouts
// Reference: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#deployment-status
func (d *DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	stdout, stderr := d.OsWriters.Stdout(), d.OsWriters.Stderr()

	if d.Infra.ServiceName == "" {
		fmt.Fprintf(stdout, "No app name in infra module. Skipping check for healthy.\n")
		return app.RolloutStatusComplete, nil
	}

	kubeClient, err := CreateKubeClient(ctx, d.Infra.Cluster.Deployer, d.Infra.Cluster)
	if err != nil {
		return "", err
	}

	deployment, err := kubeClient.AppsV1().Deployments(d.Infra.ServiceNamespace).Get(ctx, d.Infra.ServiceName, meta_v1.GetOptions{})
	if err != nil {
		return app.RolloutStatusUnknown, err
	}
	d.startDeployment.Do(func() {
		d.numDesired = 1
		if deployment.Spec.Replicas != nil {
			d.numDesired = int(*deployment.Spec.Replicas)
		}
		fmt.Fprintf(stdout, "Deploying %d replicas\n", d.numDesired)
	})

	switch reference {
	case DeployReferenceNoop:
		fmt.Fprintln(stdout, "Deployment was not changed. Skipping.")
		return app.RolloutStatusComplete, nil
	default:
		if err := d.verifyRevision(deployment, reference); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return app.RolloutStatusFailed, nil
		}
	}

	rolloutStatus, err := k8s.MapRolloutStatus(*deployment)
	if err != nil {
		return rolloutStatus, err
	}
	if rolloutStatus == app.RolloutStatusUnknown || rolloutStatus == app.RolloutStatusComplete {
		// We don't want to spit out information about replicas if the rollout is completed or unknown
		return rolloutStatus, nil
	}

	summaries := make([]string, 0)
	status := deployment.Status
	summaries = append(summaries, fmt.Sprintf("%d ready", status.AvailableReplicas))
	if status.UpdatedReplicas > 0 {
		summaries = append(summaries, fmt.Sprintf("%d up-to-date", status.UpdatedReplicas))
	}
	if status.AvailableReplicas > 0 {
		summaries = append(summaries, fmt.Sprintf("%d available", status.AvailableReplicas))
	}

	fmt.Fprintf(stdout, "%d replicas to rollout (%s)\n", d.numDesired, strings.Join(summaries, ", "))
	return rolloutStatus, nil
}

func (d *DeployStatusGetter) verifyRevision(deployment *v1.Deployment, expectedRevision string) error {
	// Since we found a deployment revision, let's verify that the latest deployment matches
	// If not, we assume that another deployment has invalidated this revision and fail this process
	latestRevision, err := k8s.Revision(deployment)
	if err != nil {
		return fmt.Errorf("Unable to identify revision on the kubernetes deployment: %s\n", err)
	} else if fmt.Sprintf("%d", latestRevision) != expectedRevision {
		return fmt.Errorf("A new deployment (revision = %d) was triggered which invalidates this deployment.", latestRevision)
	}
	return nil
}
