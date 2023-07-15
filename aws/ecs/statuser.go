package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"time"
)

var (
	// Explanations provides plain-english explanations for a task status
	// See https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-lifecycle.html
	Explanations = map[string]string{
		"PROVISIONING":   "Creating network resources",
		"PENDING":        "Provisioning compute resources",
		"RUNNING":        "Alive",
		"DEACTIVATING":   "Draining load balancer",
		"STOPPING":       "Stopping containers",
		"DEPROVISIONING": "Deleting network resources",
		"STOPPED":        "Dead",
		"DELETED":        "Deleted",
	}
)

const (
	ExplanationPullingImage            = "Pulling image"
	ExplanationRegisteringLoadBalancer = "Registering with load balancer"
)

type Status struct {
	Tasks []StatusTask `json:"tasks"`
}

type StatusTask struct {
	TaskId            string                `json:"taskId"`
	StartedAt         *time.Time            `json:"startedAt"`
	StoppedAt         *time.Time            `json:"stoppedAt"`
	StoppedReason     string                `json:"stoppedReason"`
	Status            string                `json:"status"`
	StatusExplanation string                `json:"statusExplanation"`
	Health            string                `json:"health"`
	Containers        []StatusTaskContainer `json:"containers"`
}

type StatusTaskContainer struct {
	Name   string                    `json:"name"`
	Status string                    `json:"status"`
	Health string                    `json:"health"`
	Ports  []StatusTaskContainerPort `json:"ports"`
}

type StatusTaskContainerPort struct {
	Protocol  string `json:"protocol"`
	IpAddress string `json:"ipAddress"`
	// HostPort refers to the external-facing port
	HostPort int32 `json:"hostPort"`
	// ContainerPort refers to the port that the container is listening
	ContainerPort int32 `json:"containerPort"`
	// HealthStatus refers to the status for an attached load balancer
	// This is "" if there is no attached load balancer
	HealthStatus string `json:"status"`
	// HealthReason refers to the detailed reason for an attached load balancer
	// This is "" if there is no attached load balancer
	// See github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types/TargetHealthReasonEnum
	HealthReason string `json:"reason"`
}

func NewStatuser(osWriters logging.OsWriters, nsConfig api.Config, appDetails app.Details) (app.Statuser, error) {
	outs, err := outputs.Retrieve[Outputs](nsConfig, appDetails.Workspace)
	if err != nil {
		return nil, err
	}

	return Statuser{
		OsWriters: osWriters,
		Details:   appDetails,
		Infra:     outs,
	}, nil
}

type Statuser struct {
	OsWriters logging.OsWriters
	Details   app.Details
	Infra     Outputs
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	st := Status{Tasks: make([]StatusTask, 0)}

	svcHealth, err := GetServiceHealth(ctx, s.Infra)
	if err != nil {
		return st, err
	}

	tasks, err := GetServiceTasks(ctx, s.Infra)
	if err != nil {
		return st, err
	} else if len(tasks) > 0 {
		return st, nil
	}

	for _, task := range tasks {
		st.Tasks = append(st.Tasks, StatusTask{
			TaskId:            "", // TODO: Get TaskId
			StartedAt:         task.StartedAt,
			StoppedAt:         task.StoppedAt,
			StoppedReason:     aws.ToString(task.StoppedReason),
			Status:            aws.ToString(task.LastStatus),
			StatusExplanation: mapTaskStatusToExplanation(task, svcHealth),
			Health:            string(task.HealthStatus),
			Containers:        mapTaskContainers(task, svcHealth),
		})
	}

	return st, nil
}

func mapTaskStatusToExplanation(task ecstypes.Task, svcHealth ServiceHealth) string {
	if task.PullStartedAt != nil && task.PullStoppedAt == nil {
		return ExplanationPullingImage
	}
	if aws.ToString(task.LastStatus) == "ACTIVATING" && len(svcHealth.LoadBalancers) > 0 {
		return ExplanationRegisteringLoadBalancer
	}

	if explanation, ok := Explanations[aws.ToString(task.LastStatus)]; ok {
		return explanation
	}

	return ""
}

func mapTaskContainers(task ecstypes.Task, svcHealth ServiceHealth) []StatusTaskContainer {
	containers := make([]StatusTaskContainer, 0)
	for _, container := range task.Containers {
		containers = append(containers, StatusTaskContainer{
			Name:   aws.ToString(container.Name),
			Status: aws.ToString(container.LastStatus),
			Health: string(container.HealthStatus),
			Ports:  mapContainerPorts(container, svcHealth),
		})
	}
	return containers
}

func mapContainerPorts(container ecstypes.Container, svcHealth ServiceHealth) []StatusTaskContainerPort {
	ports := make([]StatusTaskContainerPort, 0)
	for _, nb := range container.NetworkBindings {
		port := StatusTaskContainerPort{
			Protocol:      string(nb.Protocol),
			IpAddress:     aws.ToString(nb.BindIP),
			HostPort:      aws.ToInt32(nb.HostPort),
			ContainerPort: aws.ToInt32(nb.ContainerPort),
		}

		tgh := svcHealth.FindByTargetId(aws.ToString(nb.BindIP))
		if tgh != nil && tgh.TargetHealth != nil {
			port.HealthStatus = string(tgh.TargetHealth.State)
			port.HealthReason = string(tgh.TargetHealth.Reason)
		}

		ports = append(ports)
	}
	return ports
}
