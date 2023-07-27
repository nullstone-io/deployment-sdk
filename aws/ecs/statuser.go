package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"log"
	"strings"
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

type StatusOverview struct {
	Deployments []StatusOverviewDeployment `json:"deployments"`
}

type StatusOverviewDeployment struct {
	Id                 string    `json:"id"`
	CreatedAt          time.Time `json:"createdAt"`
	Status             string    `json:"status"`
	RolloutState       string    `json:"rolloutState"`
	RolloutStateReason string    `json:"rolloutStateReason"`
	DesiredCount       int32     `json:"desiredCount"`
	PendingCount       int32     `json:"pendingCount"`
	RunningCount       int32     `json:"runningCount"`
	FailedCount        int32     `json:"failedCount"`
}

type Status struct {
	Tasks []StatusTask `json:"tasks"`
}

type StatusTask struct {
	Id                string                `json:"id"`
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

func (s Statuser) StatusOverview(ctx context.Context) (any, error) {
	so := StatusOverview{Deployments: make([]StatusOverviewDeployment, 0)}

	svc, err := GetService(ctx, s.Infra)
	if err != nil {
		return so, err
	} else if svc == nil {
		return so, nil
	}

	for _, deployment := range svc.Deployments {
		so.Deployments = append(so.Deployments, StatusOverviewDeployment{
			Id:                 aws.ToString(deployment.Id),
			CreatedAt:          aws.ToTime(deployment.CreatedAt),
			Status:             aws.ToString(deployment.Status),
			RolloutState:       string(deployment.RolloutState),
			RolloutStateReason: aws.ToString(deployment.RolloutStateReason),
			DesiredCount:       deployment.DesiredCount,
			PendingCount:       deployment.PendingCount,
			RunningCount:       deployment.RunningCount,
			FailedCount:        deployment.FailedTasks,
		})
	}
	return so, nil
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	st := Status{Tasks: make([]StatusTask, 0)}
	if s.Infra.ServiceName == "" {
		// TODO: Add support for Nullstone tasks (apps that aren't long-running)
		return st, nil
	}

	svcHealth, err := GetServiceHealth(ctx, s.Infra)
	if err != nil {
		return st, err
	}

	tasks, err := GetServiceTasks(ctx, s.Infra)
	if err != nil {
		return st, err
	}

	for _, task := range tasks {
		st.Tasks = append(st.Tasks, StatusTask{
			Id:                parseTaskId(task.TaskArn),
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

func parseTaskId(taskArn *string) string {
	if taskArn == nil {
		return ""
	}
	arn := *taskArn
	return arn[strings.LastIndex(arn, "/")+1:]
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
	log.Printf("DEBUG: network_bindings: %#v\n", container.NetworkBindings)
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
