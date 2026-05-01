package k8s

import (
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// AppStatusJobExecutionPhase mirrors the K8sJobExecutionPhase TS enum so
// downstream consumers (Razor) can switch on the same string values.
type AppStatusJobExecutionPhase string

const (
	JobPhaseQueued    AppStatusJobExecutionPhase = "Queued"
	JobPhaseRunning   AppStatusJobExecutionPhase = "Running"
	JobPhaseSucceeded AppStatusJobExecutionPhase = "Succeeded"
	JobPhaseFailed    AppStatusJobExecutionPhase = "Failed"
)

// AppStatusJobExecution is one batchv1.Job folded into the shape Razor expects.
// Mirrors the K8sJobExecution TS type in razor/src/models/app_status_k8s_job.ts.
type AppStatusJobExecution struct {
	Name        string                     `json:"name"`
	ExecutionId string                     `json:"executionId"`
	AppVersion  string                     `json:"appVersion,omitempty"`
	Phase       AppStatusJobExecutionPhase `json:"phase"`
	ScheduledAt *time.Time                 `json:"scheduledAt,omitempty"`
	StartedAt   *time.Time                 `json:"startedAt,omitempty"`
	CompletedAt *time.Time                 `json:"completedAt,omitempty"`
	ExitReason  string                     `json:"exitReason,omitempty"`
}

// AppStatusJobSummary aggregates Job phases for the StatusOverview view.
// Created is the total number of Jobs returned (i.e. all created executions).
type AppStatusJobSummary struct {
	Created    int `json:"created"`
	InProgress int `json:"inProgress"`
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
}

// AppStatusJobExecutionFromK8s folds a single batchv1.Job into the execution
// record. Phase is derived from conditions first, then active/succeeded/failed
// counters as a fallback for pre-condition-set Jobs.
func AppStatusJobExecutionFromK8s(job batchv1.Job) AppStatusJobExecution {
	exec := AppStatusJobExecution{
		Name:        job.Name,
		ExecutionId: string(job.UID),
		AppVersion:  job.Labels[StandardVersionLabel],
		Phase:       jobPhase(job),
		ScheduledAt: timePtr(job.CreationTimestamp.Time),
	}
	if job.Status.StartTime != nil {
		exec.StartedAt = &job.Status.StartTime.Time
	}
	if job.Status.CompletionTime != nil {
		exec.CompletedAt = &job.Status.CompletionTime.Time
	}
	// On failure, surface the condition reason ("BackoffLimitExceeded",
	// "DeadlineExceeded", "PodFailurePolicy"...) so the UI has something to render.
	if exec.Phase == JobPhaseFailed {
		if c := jobCondition(job, batchv1.JobFailed); c != nil {
			exec.ExitReason = c.Reason
			if c.Message != "" {
				exec.ExitReason = c.Reason + ": " + c.Message
			}
			if exec.CompletedAt == nil && !c.LastTransitionTime.Time.IsZero() {
				t := c.LastTransitionTime.Time
				exec.CompletedAt = &t
			}
		}
	}
	return exec
}

// jobPhase classifies a Job into our 4-state enum. Conditions are authoritative
// when present; otherwise we infer from the active/succeeded counters.
func jobPhase(job batchv1.Job) AppStatusJobExecutionPhase {
	if c := jobCondition(job, batchv1.JobFailed); c != nil && c.Status == corev1.ConditionTrue {
		return JobPhaseFailed
	}
	if c := jobCondition(job, batchv1.JobComplete); c != nil && c.Status == corev1.ConditionTrue {
		return JobPhaseSucceeded
	}
	if job.Status.Active > 0 || job.Status.StartTime != nil {
		return JobPhaseRunning
	}
	return JobPhaseQueued
}

func jobCondition(job batchv1.Job, t batchv1.JobConditionType) *batchv1.JobCondition {
	for i := range job.Status.Conditions {
		if job.Status.Conditions[i].Type == t {
			return &job.Status.Conditions[i]
		}
	}
	return nil
}

// AppStatusJobSummaryFromK8s aggregates a list of Jobs into the overview counters.
// Created counts every Job; the others bucket by derived phase.
func AppStatusJobSummaryFromK8s(jobs []batchv1.Job) AppStatusJobSummary {
	summary := AppStatusJobSummary{Created: len(jobs)}
	for _, job := range jobs {
		switch jobPhase(job) {
		case JobPhaseRunning, JobPhaseQueued:
			summary.InProgress++
		case JobPhaseSucceeded:
			summary.Succeeded++
		case JobPhaseFailed:
			summary.Failed++
		}
	}
	return summary
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
