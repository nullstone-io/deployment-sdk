package ecs

import (
	"context"
	"fmt"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"strings"
)

func NewDeployStatusGetter(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return DeployStatusGetter{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type DeployStatusGetter struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (d DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	deployment, err := GetDeployment(ctx, d.Infra, reference)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}
	rolloutStatus := d.mapRolloutStatus(deployment)
	tasks, err := GetServiceTasks(ctx, d.Infra)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}
	d.logTasks(reference, tasks)

	return rolloutStatus, nil
}

func (d DeployStatusGetter) logTasks(reference string, tasks []ecstypes.Task) {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	var previousTasks, currentTasks []ecstypes.Task
	for _, task := range tasks {
		if *task.StartedBy == reference {
			currentTasks = append(currentTasks, task)
		} else {
			previousTasks = append(previousTasks, task)
		}
	}

	for _, task := range currentTasks {
		fmt.Fprintf(stdout, "\t\tTask %s %s\n", derefString(task.TaskArn), strings.ToLower(derefString(task.LastStatus)))
	}
	fmt.Fprintf(stdout, "\t%d existing tasks to shutdown.\n", len(previousTasks))
	for _, task := range previousTasks {
		fmt.Fprintf(stdout, "\t\tTask %s from deployment %s %s\n", derefString(task.TaskArn), derefString(task.StartedBy), strings.ToLower(derefString(task.LastStatus)))
	}
}

func (d DeployStatusGetter) mapRolloutStatus(deployment *ecstypes.Deployment) app.RolloutStatus {
	stdout, _ := d.OsWriters.Stdout(), d.OsWriters.Stderr()
	fmt.Fprintf(stdout, "%s\n", *deployment.RolloutStateReason)
	if deployment.RunningCount == deployment.DesiredCount {
		fmt.Fprintf(stdout, "\tAll %d tasks are running.\n", deployment.RunningCount)
	} else if deployment.DesiredCount > 0 {
		fmt.Fprintf(stdout, "\t%d out of %d tasks are running.\n", deployment.RunningCount, deployment.DesiredCount)
	} else {
		fmt.Fprintf(stdout, "\tNot attempting to start any tasks.\n")
	}

	status := app.RolloutStatusUnknown
	if deployment.RolloutState == "IN_PROGRESS" {
		status = app.RolloutStatusInProgress
	} else if deployment.RolloutState == "COMPLETED" {
		status = app.RolloutStatusComplete
	} else if deployment.RolloutState == "FAILED" {
		status = app.RolloutStatusFailed
	}
	return status
}

func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
