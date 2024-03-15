package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
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

type StatusTask struct {
	Id                 string                `json:"id"`
	CreatedAt          *time.Time            `json:"createdAt"`
	StartedBy          string                `json:"startedBy"`
	Connectivity       ecstypes.Connectivity `json:"connectivity"`
	ConnectivityAt     *time.Time            `json:"connectivityAt"`
	PullStartedAt      *time.Time            `json:"pullStartedAt"`
	PullStoppedAt      *time.Time            `json:"pullStoppedAt"`
	StartedAt          *time.Time            `json:"startedAt"`
	ExecutionStoppedAt *time.Time            `json:"executionStoppedAt"`
	StoppingAt         *time.Time            `json:"stoppingAt"`
	StoppedAt          *time.Time            `json:"stoppedAt"`
	StoppedReason      string                `json:"stoppedReason"`
	StopCode           ecstypes.TaskStopCode `json:"stopCode"`
	Status             string                `json:"status"`
	StatusExplanation  string                `json:"statusExplanation"`
	Health             string                `json:"health"`
	Containers         []StatusTaskContainer `json:"containers"`
}

func StatusTaskFromEcsTask(task ecstypes.Task) StatusTask {
	containers := make([]StatusTaskContainer, 0)
	for _, container := range task.Containers {
		containers = append(containers, StatusTaskContainerFromEcs(container))
	}

	return StatusTask{
		Id:                 parseTaskId(task.TaskArn),
		CreatedAt:          task.CreatedAt,
		StartedBy:          aws.ToString(task.StartedBy),
		Connectivity:       task.Connectivity,
		ConnectivityAt:     task.ConnectivityAt,
		PullStartedAt:      task.PullStartedAt,
		PullStoppedAt:      task.PullStoppedAt,
		StartedAt:          task.StartedAt,
		ExecutionStoppedAt: task.ExecutionStoppedAt,
		StoppingAt:         task.StoppingAt,
		StoppedAt:          task.StoppedAt,
		StoppedReason:      aws.ToString(task.StoppedReason),
		StopCode:           task.StopCode,
		Status:             aws.ToString(task.LastStatus),
		Health:             string(task.HealthStatus),
		Containers:         containers,
	}
}

func parseTaskId(taskArn *string) string {
	if taskArn == nil {
		return ""
	}
	return (*taskArn)[strings.LastIndex(*taskArn, "/")+1:]
}

// Enrich pulls information about the Service and TaskDefinition to build more depth to the data
// - Builds an explanation for StatusTask.Status
// - Adds port information to each StatusTaskContainer in StatusTask.Containers
func (t *StatusTask) Enrich(lbs StatusLoadBalancers, taskDef *ecstypes.TaskDefinition) {
	t.StatusExplanation = mapTaskStatusToExplanation(*t, lbs)

	for i, container := range t.Containers {
		updated := container
		updated.Enrich(lbs, taskDef)
		t.Containers[i] = updated
	}
}

type StatusTaskContainer struct {
	Name              string                      `json:"name"`
	Status            string                      `json:"status"`
	Health            string                      `json:"health"`
	Ports             []StatusTaskContainerPort   `json:"ports"`
	ExitCode          *int32                      `json:"exitCode"`
	Reason            string                      `json:"reason"`
	NetworkBindings   []ecstypes.NetworkBinding   `json:"networkBindings"`
	NetworkInterfaces []ecstypes.NetworkInterface `json:"networkInterfaces"`
}

func StatusTaskContainerFromEcs(container ecstypes.Container) StatusTaskContainer {
	return StatusTaskContainer{
		Name:              aws.ToString(container.Name),
		Status:            aws.ToString(container.LastStatus),
		Health:            string(container.HealthStatus),
		ExitCode:          container.ExitCode,
		Reason:            aws.ToString(container.Reason),
		NetworkBindings:   container.NetworkBindings,
		NetworkInterfaces: container.NetworkInterfaces,
	}
}

func (c *StatusTaskContainer) Enrich(lbs StatusLoadBalancers, taskDef *ecstypes.TaskDefinition) {
	containerDef := findContainerDefinition(c.Name, taskDef)
	if containerDef == nil {
		return
	}

	for _, portMapping := range containerDef.PortMappings {
		stcp := StatusTaskContainerPortFromEcs(*c, portMapping)
		stcp.Enrich(lbs)
		c.Ports = append(c.Ports, stcp)
	}
}

func findContainerDefinition(containerName string, taskDef *ecstypes.TaskDefinition) *ecstypes.ContainerDefinition {
	if taskDef == nil {
		return nil
	}
	for _, def := range taskDef.ContainerDefinitions {
		if aws.ToString(def.Name) == containerName {
			return &def
		}
	}
	return nil
}

func mapTaskStatusToExplanation(task StatusTask, lbs StatusLoadBalancers) string {
	if task.PullStartedAt != nil && task.PullStoppedAt == nil {
		return ExplanationPullingImage
	}
	if task.Status == "ACTIVATING" && len(lbs) > 0 {
		return ExplanationRegisteringLoadBalancer
	}

	if explanation, ok := Explanations[task.Status]; ok {
		return explanation
	}

	return ""
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

func StatusTaskContainerPortFromEcs(container StatusTaskContainer, mapping ecstypes.PortMapping) StatusTaskContainerPort {
	var ipAddress string
	for _, ni := range container.NetworkInterfaces {
		if ni.PrivateIpv4Address != nil {
			ipAddress = *ni.PrivateIpv4Address
			break
		} else if ni.Ipv6Address != nil {
			ipAddress = *ni.Ipv6Address
			break
		}
	}

	return StatusTaskContainerPort{
		Protocol:      string(mapping.Protocol),
		IpAddress:     ipAddress,
		HostPort:      aws.ToInt32(mapping.HostPort),
		ContainerPort: aws.ToInt32(mapping.ContainerPort),
	}
}

func (p *StatusTaskContainerPort) Enrich(lbs StatusLoadBalancers) {
	targets := lbs.FindTargetsById(p.IpAddress)
	// NOTE: There could be multiple load balancers attached
	// For now, we're just going to grab the health of the first one
	if len(targets) > 0 {
		p.HealthStatus = string(targets[0].HealthState)
		p.HealthReason = string(targets[0].HealthReason)
	}
}
