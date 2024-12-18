package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	"strconv"
)

func VerifyRevision(deployment *appsv1.Deployment, reference string, stdout io.Writer) app.RolloutStatus {
	latestRevision, err := Revision(deployment)
	if err != nil {
		fmt.Fprintf(stdout, "Unable to identify revision on the kubernetes deployment: %s\n", err)
		return app.RolloutStatusFailed
	}

	expectedRevision, err := strconv.ParseInt(reference, 10, 64)
	if err != nil {
		fmt.Fprintln(stdout, "Invalid deployment reference. Expected a deployment revision number.")
		return app.RolloutStatusFailed
	}

	if latestRevision < expectedRevision {
		// If the deployment has a revision smaller than the expected, it must not be in the k8s cluster yet
		fmt.Fprintln(stdout, "Waiting for deployment to start.")
		return app.RolloutStatusPending
	} else if latestRevision > expectedRevision {
		// If the deployment has a revision larger than the expected, there must be a new deployment that invalidates this one
		fmt.Fprintf(stdout, "A new deployment (revision = %d) was triggered which invalidates this deployment.\n", latestRevision)
		return app.RolloutStatusFailed
	}

	return app.RolloutStatusInProgress
}
