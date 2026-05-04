package ecs

import (
	"time"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// EcsJobExecutionPhase mirrors AppStatusJobExecutionPhase on the K8s side so
// downstream consumers (Razor) can switch on the same string values across
// providers. See k8s/status_job.go.
type EcsJobExecutionPhase string

const (
	EcsJobPhaseQueued    EcsJobExecutionPhase = "Queued"
	EcsJobPhaseRunning   EcsJobExecutionPhase = "Running"
	EcsJobPhaseSucceeded EcsJobExecutionPhase = "Succeeded"
	EcsJobPhaseFailed    EcsJobExecutionPhase = "Failed"
)

// EcsJobExecution is one historical run of an ECS task in a job workspace
// (workspaces with no service — RunTask one-offs). Folds a StatusTask into
// the shape Razor's EcsJobExecution.vue card consumes. Populated only when
// Status.IsJob.
type EcsJobExecution struct {
	ExecutionId            string                `json:"executionId"`
	TaskArn                string                `json:"taskArn"`
	TaskDefinitionFamily   string                `json:"taskDefinitionFamily"`
	TaskDefinitionRevision int32                 `json:"taskDefinitionRevision"`
	AppVersion             string                `json:"appVersion,omitempty"`
	Phase                  EcsJobExecutionPhase  `json:"phase"`
	StartedAt              *time.Time            `json:"startedAt,omitempty"`
	StoppedAt              *time.Time            `json:"stoppedAt,omitempty"`
	StoppedReason          string                `json:"stoppedReason,omitempty"`
	StopCode               ecstypes.TaskStopCode `json:"stopCode,omitempty"`
	EnableExecuteCommand   bool                  `json:"enableExecuteCommand"`
	Containers             []StatusTaskContainer `json:"containers"`
}

// EcsJobExecutionFromStatusTask folds an enriched StatusTask plus its
// resolved appVersion (from the task definition's nullstone version tag)
// into the execution shape.
func EcsJobExecutionFromStatusTask(task StatusTask, appVersion string) EcsJobExecution {
	return EcsJobExecution{
		ExecutionId:            task.Id,
		TaskArn:                task.TaskArn,
		TaskDefinitionFamily:   task.TaskDefinitionFamily,
		TaskDefinitionRevision: task.TaskDefinitionRevision,
		AppVersion:             appVersion,
		Phase:                  deriveEcsJobPhase(task),
		StartedAt:              task.StartedAt,
		StoppedAt:              task.StoppedAt,
		StoppedReason:          task.StoppedReason,
		StopCode:               task.StopCode,
		EnableExecuteCommand:   task.EnableExecuteCommand,
		Containers:             task.Containers,
	}
}

// deriveEcsJobPhase classifies a task into our 4-state phase enum. Container
// exit codes are authoritative when present; otherwise StopCode is the fallback.
//
// Phase mapping:
//   - PROVISIONING / PENDING       → Queued
//   - ACTIVATING / RUNNING / *ING  → Running (ACTIVATING/DEACTIVATING/STOPPING/DEPROVISIONING are mid-lifecycle)
//   - STOPPED + any non-zero exit  → Failed
//   - STOPPED + StopCode == TaskFailedToStart → Failed (the task never ran)
//   - STOPPED + all exits 0        → Succeeded
//   - STOPPED + no exit info, StopCode UserInitiated/EssentialContainerExited → Succeeded
//   - STOPPED + no exit info, anything else (Spot, TerminationNotice, ...)    → Failed
func deriveEcsJobPhase(task StatusTask) EcsJobExecutionPhase {
	switch task.Status {
	case "PROVISIONING", "PENDING":
		return EcsJobPhaseQueued
	case "STOPPED":
		return classifyStoppedPhase(task)
	default:
		// ACTIVATING, RUNNING, DEACTIVATING, STOPPING, DEPROVISIONING — any
		// non-terminal state surfaces as Running so the UI shows a spinner.
		return EcsJobPhaseRunning
	}
}

func classifyStoppedPhase(task StatusTask) EcsJobExecutionPhase {
	if task.StopCode == ecstypes.TaskStopCodeTaskFailedToStart {
		return EcsJobPhaseFailed
	}
	hasExitCode := false
	for _, c := range task.Containers {
		if c.ExitCode == nil {
			continue
		}
		hasExitCode = true
		if *c.ExitCode != 0 {
			return EcsJobPhaseFailed
		}
	}
	if hasExitCode {
		return EcsJobPhaseSucceeded
	}
	// No container exit codes available — fall back to StopCode.
	switch task.StopCode {
	case ecstypes.TaskStopCodeUserInitiated, ecstypes.TaskStopCodeEssentialContainerExited:
		return EcsJobPhaseSucceeded
	default:
		// SpotInterruption, TerminationNotice, ServiceSchedulerInitiated, or empty.
		return EcsJobPhaseFailed
	}
}
