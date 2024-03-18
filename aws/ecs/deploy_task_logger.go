package ecs

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/display"
	"github.com/nullstone-io/deployment-sdk/logging"
	"strings"
	"time"
)

type deployTaskLoggers map[string]*deployTaskLogger

func (l deployTaskLoggers) Refresh(ctx context.Context, osWriters logging.OsWriters, infra Outputs, deploymentId string, lbs StatusLoadBalancers, taskDef *ecstypes.TaskDefinition) error {
	// 1. Look for new task arns and collate them into the existing list of task loggers
	taskArns, err := GetAllDeploymentTaskArns(ctx, infra, deploymentId)
	if err != nil {
		return fmt.Errorf("unable to retrieve new deployment tasks: %w", err)
	}
	taskArnsToInit := map[string]bool{}
	for _, taskArn := range taskArns {
		if _, ok := l[taskArn]; !ok {
			taskArnsToInit[taskArn] = true
			l[taskArn] = newDeployTaskLogger(osWriters, taskArn)
		}
	}

	// 2. Collect the details of all tasks via a single DescribeTasks call
	tasks, err := GetTasksWithDetail(ctx, infra, l.getAllTaskArns())
	if err != nil {
		return fmt.Errorf("unable to refresh task detail: %w", err)
	}

	// 3. Feed that detail into the task loggers (via Init or Refresh)
	for _, task := range tasks {
		taskArn := *task.TaskArn
		taskLogger, _ := l[taskArn] // This should always contain a task logger
		if _, needsInit := taskArnsToInit[*task.TaskArn]; needsInit {
			taskLogger.Init(task)
		} else {
			taskLogger.Refresh(task, lbs, taskDef)
		}
	}

	return nil
}

func (l deployTaskLoggers) getAllTaskArns() []string {
	all := make([]string, 0)
	for _, tl := range l {
		all = append(all, tl.TaskArn)
	}
	return all
}

type deployTaskLogger struct {
	TaskId    string
	TaskArn   string
	OsWriters logging.OsWriters

	task       *StatusTask
	containers deployContainerLoggers
}

func newDeployTaskLogger(osWriters logging.OsWriters, taskArn string) *deployTaskLogger {
	taskId := parseTaskId(&taskArn)
	dtw := &deployTaskLogger{
		TaskId:     taskId,
		TaskArn:    taskArn,
		OsWriters:  osWriters,
		containers: deployContainerLoggers{},
	}
	return dtw
}

func (l *deployTaskLogger) Init(task ecstypes.Task) {
	st := StatusTaskFromEcsTask(task)
	l.task = &st
	if l.task != nil {
		createdAt := aws.ToTime(l.task.CreatedAt)
		l.log(createdAt, "Created task")
		l.log(createdAt, "Provisioning compute resources")
	}
}

func (l *deployTaskLogger) Refresh(updated ecstypes.Task, lbs StatusLoadBalancers, taskDef *ecstypes.TaskDefinition) {
	previous := l.task
	st := StatusTaskFromEcsTask(updated)
	st.Enrich(lbs, taskDef)
	l.task = &st

	l.containers.Refresh(l.OsWriters, st.Containers, l.TaskId)

	l.comparePrevious(previous)
}

func (l *deployTaskLogger) comparePrevious(previous *StatusTask) {
	if previous == nil || l.task == nil {
		return
	}

	// Resources:
	// https://containersonaws.com/visuals/ecs-task-lifecycle/
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/stopped-task-error-codes.html

	now := time.Now()
	if l.task.Status != previous.Status {
		explanation := Explanations[l.task.Status]
		switch l.task.Status {
		case "PENDING":
			l.log(now, explanation)
		case "ACTIVATING":
			l.log(now, explanation)
		case "RUNNING": // Handled below
		case "DEACTIVATING":
			l.log(now, explanation)
		case "STOPPING": // Handled below
		case "DEPROVISIONING":
			l.log(now, explanation)
		case "STOPPED": // Handled below
		case "DELETED":
			l.log(now, explanation)
		}
	}

	if at := aws.ToTime(l.task.StartedAt); at != aws.ToTime(previous.StartedAt) {
		l.log(at, "Task started")
	}
	if at := aws.ToTime(l.task.PullStartedAt); at != aws.ToTime(previous.PullStartedAt) {
		l.log(at, "Pulling image")
	}
	if at := aws.ToTime(l.task.PullStoppedAt); at != aws.ToTime(previous.PullStoppedAt) {
		// PullStoppedAt refers to the image pull stopping for failure and success
		// We're only going to log "Image pulled" when it was successful
		if !strings.HasPrefix(l.task.StoppedReason, "CannotPullContainerError") {
			l.log(at, "Image pulled")
		}
	}
	if at := aws.ToTime(l.task.StoppingAt); at != aws.ToTime(previous.StoppingAt) {
		l.log(at, "Task stopping")
		if l.task.StopCode != previous.StopCode {
			l.log(aws.ToTime(l.task.StoppingAt), string(l.task.StopCode))
		}
		if l.task.StoppedReason != previous.StoppedReason {
			l.log(aws.ToTime(l.task.StoppingAt), l.task.StoppedReason)
		}
	}
	if at := aws.ToTime(l.task.StoppedAt); at != aws.ToTime(previous.StoppedAt) {
		l.log(at, "Task stopped")
	}

	//HealthStatus,
}

func (l *deployTaskLogger) log(at time.Time, msg string) {
	fmt.Fprintf(l.OsWriters.Stdout(), "%s [%s] %s\n", display.FormatTime(at), l.TaskId, msg)
}
