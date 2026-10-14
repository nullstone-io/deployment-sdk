package ecs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// taskStateChangeEvent is an EventBridge "ECS Task State Change" event as EventBridge writes it to a
// CloudWatch log group. Only the fields the status path needs are declared; the rest of the event
// (attachments, attributes, overrides, ...) is ignored.
// See https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_task_events.html
type taskStateChangeEvent struct {
	Time   time.Time             `json:"time"`
	Detail taskStateChangeDetail `json:"detail"`
}

// taskStateChangeDetail is the ECS Task object carried in the event, in camelCase JSON.
// Timestamps are ISO 8601 strings, which time.Time decodes directly.
type taskStateChangeDetail struct {
	TaskArn              string                     `json:"taskArn"`
	TaskDefinitionArn    string                     `json:"taskDefinitionArn"`
	ClusterArn           string                     `json:"clusterArn"`
	Group                string                     `json:"group"`
	LastStatus           string                     `json:"lastStatus"`
	DesiredStatus        string                     `json:"desiredStatus"`
	StartedBy            string                     `json:"startedBy"`
	EnableExecuteCommand bool                       `json:"enableExecuteCommand"`
	Version              int64                      `json:"version"`
	Connectivity         string                     `json:"connectivity"`
	ConnectivityAt       *time.Time                 `json:"connectivityAt"`
	CreatedAt            *time.Time                 `json:"createdAt"`
	PullStartedAt        *time.Time                 `json:"pullStartedAt"`
	PullStoppedAt        *time.Time                 `json:"pullStoppedAt"`
	StartedAt            *time.Time                 `json:"startedAt"`
	StoppingAt           *time.Time                 `json:"stoppingAt"`
	StoppedAt            *time.Time                 `json:"stoppedAt"`
	ExecutionStoppedAt   *time.Time                 `json:"executionStoppedAt"`
	StoppedReason        string                     `json:"stoppedReason"`
	StopCode             string                     `json:"stopCode"`
	Containers           []taskStateChangeContainer `json:"containers"`
}

type taskStateChangeContainer struct {
	Name       string `json:"name"`
	Image      string `json:"image"`
	LastStatus string `json:"lastStatus"`
	ExitCode   *int32 `json:"exitCode"`
	Reason     string `json:"reason"`
}

func parseTaskStateChangeEvent(message string) (taskStateChangeEvent, error) {
	var event taskStateChangeEvent
	if err := json.Unmarshal([]byte(message), &event); err != nil {
		return event, fmt.Errorf("invalid task state change event: %w", err)
	}
	if event.Detail.TaskArn == "" {
		return event, fmt.Errorf("invalid task state change event: missing detail.taskArn")
	}
	return event, nil
}

// ToEcsTask converts the event detail into the SDK task type so history can flow through the same
// StatusTask / EcsJobExecution pipeline as tasks returned by DescribeTasks.
func (d taskStateChangeDetail) ToEcsTask() ecstypes.Task {
	containers := make([]ecstypes.Container, 0, len(d.Containers))
	for _, c := range d.Containers {
		containers = append(containers, ecstypes.Container{
			Name:       aws.String(c.Name),
			Image:      aws.String(c.Image),
			LastStatus: aws.String(c.LastStatus),
			ExitCode:   c.ExitCode,
			Reason:     aws.String(c.Reason),
		})
	}
	return ecstypes.Task{
		TaskArn:              aws.String(d.TaskArn),
		TaskDefinitionArn:    aws.String(d.TaskDefinitionArn),
		ClusterArn:           aws.String(d.ClusterArn),
		Group:                aws.String(d.Group),
		LastStatus:           aws.String(d.LastStatus),
		DesiredStatus:        aws.String(d.DesiredStatus),
		StartedBy:            aws.String(d.StartedBy),
		EnableExecuteCommand: d.EnableExecuteCommand,
		Version:              d.Version,
		Connectivity:         ecstypes.Connectivity(d.Connectivity),
		ConnectivityAt:       d.ConnectivityAt,
		CreatedAt:            d.CreatedAt,
		PullStartedAt:        d.PullStartedAt,
		PullStoppedAt:        d.PullStoppedAt,
		StartedAt:            d.StartedAt,
		StoppingAt:           d.StoppingAt,
		StoppedAt:            d.StoppedAt,
		ExecutionStoppedAt:   d.ExecutionStoppedAt,
		StoppedReason:        aws.String(d.StoppedReason),
		StopCode:             ecstypes.TaskStopCode(d.StopCode),
		Containers:           containers,
	}
}
