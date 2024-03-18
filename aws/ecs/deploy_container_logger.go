package ecs

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/nullstone-io/deployment-sdk/display"
	"github.com/nullstone-io/deployment-sdk/logging"
	"slices"
	"time"
)

type deployContainerLoggers map[string]*deployContainerLogger

func (s deployContainerLoggers) Refresh(osWriters logging.OsWriters, containers []StatusTaskContainer, taskId string) {
	for _, container := range containers {
		containerLogger, ok := s[container.Name]
		if !ok {
			containerLogger = newDeployContainerLogger(osWriters, taskId, container.Name)
			containerLogger.Init(container)
			s[container.Name] = containerLogger
		} else {
			containerLogger.Refresh(container)
		}
	}
}

type deployContainerLogger struct {
	OsWriters     logging.OsWriters
	TaskId        string
	ContainerName string

	container *StatusTaskContainer
	ports     []StatusTaskContainerPort
}

func newDeployContainerLogger(osWriters logging.OsWriters, taskId, containerName string) *deployContainerLogger {
	return &deployContainerLogger{
		OsWriters:     osWriters,
		TaskId:        taskId,
		ContainerName: containerName,
	}
}

func (l *deployContainerLogger) Init(container StatusTaskContainer) {}

func (l *deployContainerLogger) Refresh(container StatusTaskContainer) {
	previous, previousPorts := l.container, l.ports
	l.container, l.ports = &container, container.Ports

	l.comparePreviousPorts(previousPorts)
	l.comparePrevious(previous)
}

func (l *deployContainerLogger) comparePreviousPorts(previous []StatusTaskContainerPort) {
	if previous == nil {
		return
	}

	now := time.Now()

	for _, curPort := range l.container.Ports {
		index := slices.IndexFunc(previous, func(port StatusTaskContainerPort) bool {
			return port.HostPort == curPort.HostPort && port.ContainerPort == curPort.ContainerPort
		})
		prefix := fmt.Sprintf("%s/%d => %d", curPort.Protocol, curPort.HostPort, curPort.ContainerPort)
		if index < 0 {
			// TODO: Init
		} else {
			prevPort := previous[index]
			if prevPort.HealthStatus != curPort.HealthStatus {
				switch elbv2types.TargetHealthStateEnum(curPort.HealthStatus) {
				case elbv2types.TargetHealthStateEnumHealthy:
					l.log(now, fmt.Sprintf("(%s) Target is healthy", prefix))
				case elbv2types.TargetHealthStateEnumUnhealthy:
					l.log(now, fmt.Sprintf("(%s) Target is unhealthy", prefix))
				default:
				}
			}
			if prevPort.HealthReason != curPort.HealthReason {
				reason := elbv2types.TargetHealthReasonEnum(curPort.HealthReason)
				explanation := LbHealthReasonExplanations[reason]
				if explanation != "" {
					l.log(now, fmt.Sprintf("(%s) %s", prefix, explanation))
				}
			}
		}
	}
}

func (l *deployContainerLogger) comparePrevious(previous *StatusTaskContainer) {
	if previous == nil {
		return
	}

	now := time.Now()
	if l.container.Health != previous.Health {
		switch types.HealthStatus(l.container.Health) {
		case types.HealthStatusHealthy:
			l.log(now, "Container is healthy")
		case types.HealthStatusUnhealthy:
			l.log(now, "Container is unhealthy")
		}
	}
	if l.container.Status != previous.Status {
		if l.container.Status != "" {
			l.log(now, fmt.Sprintf("Container transitioned to %s", l.container.Status))
		}
	}
	if l.container.Reason != previous.Reason {
		l.log(now, l.container.Reason)
	}
	if previous.ExitCode == nil && l.container.ExitCode != nil {
		l.log(now, fmt.Sprintf("Container exited (code = %d)", *l.container.ExitCode))
	}
}

func (l *deployContainerLogger) log(at time.Time, msg string) {
	fmt.Fprintf(l.OsWriters.Stdout(), "%s [%s/%s] %s\n", display.FormatTime(at), l.TaskId, l.ContainerName, msg)
}

var (
	// See elbv2types.TargetHealth.Reason and elbv2types.TargetHealth.TargetHealthReasonEnum
	LbHealthReasonExplanations = map[elbv2types.TargetHealthReasonEnum]string{
		elbv2types.TargetHealthReasonEnumRegistrationInProgress:   "The target is in the process of being registered with the load balancer.",
		elbv2types.TargetHealthReasonEnumInitialHealthChecking:    "The load balancer is still sending the target the minimum number of health checks required to determine its health status.",
		elbv2types.TargetHealthReasonEnumResponseCodeMismatch:     "The health checks did not return an expected HTTP code.",
		elbv2types.TargetHealthReasonEnumTimeout:                  "The health check requests timed out.",
		elbv2types.TargetHealthReasonEnumFailedHealthChecks:       "The load balancer received an error while establishing a connection to the target or the target response was malformed.",
		elbv2types.TargetHealthReasonEnumNotRegistered:            "The target is not registered with the target group.",
		elbv2types.TargetHealthReasonEnumNotInUse:                 "The target group is not used by any load balancer or the target is in an Availability Zone that is not enabled for its load balancer.",
		elbv2types.TargetHealthReasonEnumDeregistrationInProgress: "The target is in the process of being deregistered and the deregistration delay period has not expired.",
		elbv2types.TargetHealthReasonEnumInvalidState:             "The target is in the stopped or terminated state.",
		elbv2types.TargetHealthReasonEnumIpUnusable:               "The target IP address is reserved for use by a load balancer.",
		elbv2types.TargetHealthReasonEnumHealthCheckDisabled:      "Health checks are disabled for the target group. Applies only to Application Load Balancers.",
		elbv2types.TargetHealthReasonEnumInternalError:            "Target health is unavailable due to an internal error.",
	}
)
