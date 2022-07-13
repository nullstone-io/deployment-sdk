package ecs

import (
	"context"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
	"strings"
)

func NewDeployStatusGetter(logger *log.Logger, nsConfig api.Config, appDetails app.Details) (app.DeployStatusGetter, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return DeployStatusGetter{
		Logger:   logger,
		NsConfig: nsConfig,
		Details:  appDetails,
		Infra:    outs,
	}, nil
}

type DeployStatusGetter struct {
	Logger   *log.Logger
	NsConfig api.Config
	Details  app.Details
	Infra    Outputs
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
	tasks, err := GetDeploymentTasks(ctx, d.Infra, reference)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}
	d.logTasks(reference, tasks)

	return rolloutStatus, nil
}

func (d DeployStatusGetter) logTasks(reference string, tasks []ecstypes.Task) {
	var previousTasks, currentTasks []ecstypes.Task
	for _, task := range tasks {
		if *task.StartedBy == reference {
			currentTasks = append(currentTasks, task)
		} else {
			previousTasks = append(previousTasks, task)
		}
	}

	for _, task := range currentTasks {
		d.Logger.Printf("\t\tTask %s %s", derefString(task.TaskArn), strings.ToLower(derefString(task.LastStatus)))
	}
	d.Logger.Printf("\t%d existing tasks to shutdown.", len(previousTasks))
	for _, task := range previousTasks {
		d.Logger.Printf("\t\tTask %s from deployment %s %s", derefString(task.TaskArn), derefString(task.StartedBy), strings.ToLower(derefString(task.LastStatus)))
	}
}

func (d DeployStatusGetter) mapRolloutStatus(deployment *ecstypes.Deployment) app.RolloutStatus {
	d.Logger.Printf("%s\n", *deployment.RolloutStateReason)
	if deployment.RunningCount == deployment.DesiredCount {
		d.Logger.Printf("\tAll %d tasks are running.\n", deployment.RunningCount)
	} else if deployment.DesiredCount > 0 {
		d.Logger.Printf("\t%d out of %d tasks are running.\n", deployment.RunningCount, deployment.DesiredCount)
	} else {
		d.Logger.Printf("\tNot attempting to start any tasks.\n")
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
