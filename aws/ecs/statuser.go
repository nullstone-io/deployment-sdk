package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
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
	CreatedAt         *time.Time            `json:"createdAt"`
	StartedBy         string                `json:"startedBy"`
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
	HealthStatus string `json:"healthStatus"`
	// HealthReason refers to the detailed reason for an attached load balancer
	// This is "" if there is no attached load balancer
	// See github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types/TargetHealthReasonEnum
	HealthReason string `json:"healthReason"`
}

func NewStatuser(osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Statuser, error) {
	outs, err := outputs.Retrieve[Outputs](source, appDetails.Workspace)
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
	if s.Infra.ServiceName == "" {
		// no service name means this is an ecs task and there are no deployments
		return so, nil
	}

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
	tasks, err := s.getTasks(ctx)
	if err != nil {
		return st, err
	}

	svcHealth, err := GetServiceHealth(ctx, s.Infra)
	if err != nil {
		return st, err
	}

	taskDefs := map[string]*ecstypes.TaskDefinition{}
	for _, task := range tasks {
		var taskDef *ecstypes.TaskDefinition
		if task.TaskDefinitionArn != nil {
			if def, ok := taskDefs[*task.TaskDefinitionArn]; ok {
				taskDef = def
			} else {
				def, err := GetTaskDefinition(ctx, s.Infra)
				if err != nil {
					return st, err
				}
				taskDefs[*task.TaskDefinitionArn] = def
				taskDef = def
			}
		}

		st.Tasks = append(st.Tasks, StatusTask{
			Id:                parseTaskId(task.TaskArn),
			CreatedAt:         task.CreatedAt,
			StartedBy:         aws.ToString(task.StartedBy),
			StartedAt:         task.StartedAt,
			StoppedAt:         task.StoppedAt,
			StoppedReason:     aws.ToString(task.StoppedReason),
			Status:            aws.ToString(task.LastStatus),
			StatusExplanation: mapTaskStatusToExplanation(task, svcHealth),
			Health:            string(task.HealthStatus),
			Containers:        mapTaskContainers(task, taskDef, svcHealth),
		})
	}

	return st, nil
}

func (s Statuser) getTasks(ctx context.Context) ([]ecstypes.Task, error) {
	if s.Infra.ServiceName == "" {
		return GetTaskFamilyTasks(ctx, s.Infra)
	} else {
		return GetServiceTasks(ctx, s.Infra)
	}
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

func mapTaskContainers(task ecstypes.Task, taskDef *ecstypes.TaskDefinition, svcHealth ServiceHealth) []StatusTaskContainer {
	containers := make([]StatusTaskContainer, 0)
	for _, container := range task.Containers {
		containers = append(containers, StatusTaskContainer{
			Name:   aws.ToString(container.Name),
			Status: aws.ToString(container.LastStatus),
			Health: string(container.HealthStatus),
			Ports:  mapContainerPorts(container, taskDef, svcHealth),
		})
	}
	return containers
}

func mapContainerPorts(container ecstypes.Container, taskDef *ecstypes.TaskDefinition, svcHealth ServiceHealth) []StatusTaskContainerPort {
	ports := make([]StatusTaskContainerPort, 0)

	containerDef := findContainerDefinition(container, taskDef)
	if containerDef == nil {
		return ports
	}
	ipAddress := ""
	if len(container.NetworkInterfaces) > 0 {
		ipAddress = aws.ToString(container.NetworkInterfaces[0].PrivateIpv4Address)
	}
	for _, mapping := range containerDef.PortMappings {
		port := StatusTaskContainerPort{
			Protocol:      string(mapping.Protocol),
			IpAddress:     ipAddress,
			HostPort:      aws.ToInt32(mapping.HostPort),
			ContainerPort: aws.ToInt32(mapping.ContainerPort),
		}

		tgh := svcHealth.FindByTargetId(ipAddress)
		if tgh != nil && tgh.TargetHealth != nil {
			port.HealthStatus = string(tgh.TargetHealth.State)
			port.HealthReason = string(tgh.TargetHealth.Reason)
		}

		ports = append(ports, port)
	}
	return ports
}

func findContainerDefinition(container ecstypes.Container, taskDef *ecstypes.TaskDefinition) *ecstypes.ContainerDefinition {
	if taskDef == nil {
		return nil
	}
	for _, def := range taskDef.ContainerDefinitions {
		if aws.ToString(def.Name) == aws.ToString(container.Name) {
			return &def
		}
	}
	return nil
}
