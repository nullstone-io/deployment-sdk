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

	sync.Once
	curRolloutStatus app.RolloutStatus
	previousTaskArns []string
	startDeployment  sync.Once
	numDesired       int
}

func (d *DeployStatusGetter) initialize(ctx context.Context, reference string) error {
	var err error
	d.Do(func() {
		var tasks []ecstypes.Task
		tasks, err = GetServiceTasks(ctx, d.Infra)
		d.previousTaskArns = make([]string, 0)
		for _, task := range tasks {
			if task.StartedBy != nil && *task.StartedBy != reference {
				d.previousTaskArns = append(d.previousTaskArns, *task.TaskArn)
			}
		}
	})
	return err
}

func (d *DeployStatusGetter) GetDeployStatus(ctx context.Context, reference string) (app.RolloutStatus, error) {
	if err := d.initialize(ctx, reference); err != nil {
		return app.RolloutStatusUnknown, err
	}

	if reference == "" {
		return app.RolloutStatusUnknown, nil
	}

	deployment, err := GetDeployment(ctx, d.Infra, reference)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}
	d.startDeployment.Do(func() {
		d.numDesired = int(deployment.DesiredCount)
		fmt.Fprintf(d.OsWriters.Stdout(), "Deploying %d tasks\n", deployment.DesiredCount)
	})
	rolloutStatus := d.mapRolloutStatus(deployment)
	if rolloutStatus == app.RolloutStatusUnknown || rolloutStatus == app.RolloutStatusComplete {
		// We don't want to spit out information about tasks if the rollout is completed or unknown
		return rolloutStatus, nil
	}

	cur, err := d.buildCurrent(ctx, reference)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}
	if cur == "" {
		cur = "0 running"
	}
	minWidth := 35
	extra := ""
	if len(cur) < minWidth {
		extra = strings.Repeat(" ", minWidth-len(cur))
	}

	prev, err := d.buildPrevious(ctx)
	if err != nil {
		return app.RolloutStatusUnknown, err
	}

	fmt.Fprintf(d.OsWriters.Stdout(), "%d tasks to add (%s)%s %d tasks to remove (%s)\n", d.numDesired, cur, extra, len(d.previousTaskArns), prev)
	return rolloutStatus, nil
}

func (d *DeployStatusGetter) buildCurrent(ctx context.Context, reference string) (string, error) {
	curTasks, err := GetDeploymentTasks(ctx, d.Infra, reference)
	if err != nil {
		return "", fmt.Errorf("error retrieving tasks in current deployment: %w", err)
	}
	return summarizeTaskStatuses(curTasks), nil
}

func (d *DeployStatusGetter) buildPrevious(ctx context.Context) (string, error) {
	prevTasks, err := DescribeTasks(ctx, d.Infra, d.previousTaskArns)
	if err != nil {
		return "", fmt.Errorf("error retrieving tasks in previous deployment: %w", err)
	}
	return summarizeTaskStatuses(prevTasks), nil
}

func (d *DeployStatusGetter) mapRolloutStatus(deployment *ecstypes.Deployment) app.RolloutStatus {
	newStatus := app.RolloutStatusUnknown
	switch deployment.RolloutState {
	case ecstypes.DeploymentRolloutStateInProgress:
		newStatus = app.RolloutStatusInProgress
	case ecstypes.DeploymentRolloutStateCompleted:
		newStatus = app.RolloutStatusComplete
	case ecstypes.DeploymentRolloutStateFailed:
		newStatus = app.RolloutStatusFailed
	}

	if d.curRolloutStatus != newStatus {
		fmt.Fprintln(d.OsWriters.Stdout(), derefString(deployment.RolloutStateReason))
	}
	d.curRolloutStatus = newStatus
	return newStatus
}

func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func summarizeTaskStatuses(tasks []ecstypes.Task) string {
	byStatus := groupBy(tasks, func(task ecstypes.Task) string { return derefString(task.LastStatus) })
	summaries := make([]string, 0)
	for status, tasks := range byStatus {
		summaries = append(summaries, fmt.Sprintf("%d %s", len(tasks), strings.ToLower(status)))
	}
	return strings.Join(summaries, ", ")
}

func groupBy[TValue any, TKey comparable](values []TValue, keyFn func(val TValue) TKey) map[TKey][]TValue {
	m := map[TKey][]TValue{}
	for _, val := range values {
		key := keyFn(val)
		if grp, ok := m[key]; !ok {
			m[key] = []TValue{val}
		} else {
			m[key] = append(grp, val)
		}
	}
	return m
}

func hasTasksNotInRunning(tasks []ecstypes.Task) bool {
	for _, task := range tasks {
		if derefString(task.LastStatus) != "RUNNING" {
			return true
		}
	}
	return false
}
