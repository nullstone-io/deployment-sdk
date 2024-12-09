package eks

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"io"
	v1 "k8s.io/api/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
	"sync"
)

func NewDeployStatusGetter(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
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

// GetDeployStatus resolves the current status of the eks deployment
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

	kubeClient, err := CreateKubeClient(ctx, d.Infra.Region, d.Infra.ClusterNamespace, d.Infra.Deployer)
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
		if ok, status := d.verifyRevision(deployment, reference, stdout); !ok {
			return status, nil
		}
	}

	rolloutStatus, err := k8s.MapRolloutStatus(*deployment)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return rolloutStatus, nil
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

func (d *DeployStatusGetter) verifyRevision(deployment *v1.Deployment, reference string, stdout io.Writer) (bool, app.RolloutStatus) {
	latestRevision, err := k8s.Revision(deployment)
	if err != nil {
		fmt.Fprintf(stdout, "Unable to identify revision on the kubernetes deployment: %s\n", err)
		return false, app.RolloutStatusFailed
	}

	expectedRevision, err := strconv.ParseInt(reference, 10, 64)
	if err != nil {
		fmt.Fprintln(stdout, "Invalid deployment reference. Expected a deployment revision number.")
		return false, app.RolloutStatusFailed
	}

	if latestRevision < expectedRevision {
		// If the deployment has a revision smaller than the expected, it must not be in the k8s cluster yet
		fmt.Fprintln(stdout, "Waiting for deployment to start.")
		return false, app.RolloutStatusInProgress
	} else if latestRevision > expectedRevision {
		// If the deployment has a revision larger than the expected, there must be a new deployment that invalidates this one
		fmt.Fprintf(stdout, "A new deployment (revision = %d) was triggered which invalidates this deployment.\n", latestRevision)
		return false, app.RolloutStatusFailed
	}

	return true, app.RolloutStatusInProgress
}
