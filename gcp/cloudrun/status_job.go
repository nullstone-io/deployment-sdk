package cloudrun

import (
	"context"
	"fmt"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/nullstone-io/deployment-sdk/docker"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
)

const (
	// maxExecutions bounds how many historical executions we surface.
	maxExecutions = 20
	// maxExecutionsWithTasks bounds the per-task fan-out — we only fetch the
	// task array for the most-recent executions to cap API calls.
	maxExecutionsWithTasks = 10
	// maxTasksPerExecution caps the task array we render for a single execution.
	maxTasksPerExecution = 500
)

func (s Statuser) statusJob(ctx context.Context) ([]JobExecution, error) {
	execClient, err := NewExecutionsClient(ctx, s.Infra.Deployer)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloud run executions client: %w", err)
	}
	defer execClient.Close()

	rawExecs := make([]*runpb.Execution, 0)
	it := execClient.ListExecutions(ctx, &runpb.ListExecutionsRequest{Parent: s.Infra.JobId})
	for len(rawExecs) < maxExecutions {
		exec, err := it.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("error listing executions: %w", err)
		}
		rawExecs = append(rawExecs, exec)
	}

	executions := make([]JobExecution, len(rawExecs))
	g, gctx := errgroup.WithContext(ctx)
	for i, exec := range rawExecs {
		i, exec := i, exec
		g.Go(func() error {
			je := s.mapExecution(exec)
			if i < maxExecutionsWithTasks {
				tasks, diag, err := s.listExecutionTasks(gctx, exec.GetName())
				if err != nil {
					return err
				}
				je.Tasks = tasks
				je.Failure = deriveExecutionFailure(je, diag)
			}
			executions[i] = je
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return executions, nil
}

func (s Statuser) mapExecution(exec *runpb.Execution) JobExecution {
	je := JobExecution{
		Name:           s.Infra.JobName,
		ExecutionId:    shortName(exec.GetName()),
		Phase:          executionPhase(exec),
		ScheduledAt:    tsToTime(exec.GetCreateTime()),
		StartedAt:      tsToTime(exec.GetStartTime()),
		CompletedAt:    tsToTime(exec.GetCompletionTime()),
		TaskCount:      exec.GetTaskCount(),
		Parallelism:    exec.GetParallelism(),
		CompletedCount: exec.GetSucceededCount(),
		RunningCount:   exec.GetRunningCount(),
		FailedCount:    exec.GetFailedCount(),
		RetryCount:     exec.GetRetriedCount(),
		Tasks:          make([]Task, 0),
	}

	if tmpl := exec.GetTemplate(); tmpl != nil {
		je.MaxRetries = tmpl.GetMaxRetries()
		_, container := GetContainerByName(tmpl.GetContainers(), s.Infra.MainContainerName)
		if container == nil && len(tmpl.GetContainers()) > 0 {
			container = tmpl.GetContainers()[0]
		}
		if container != nil {
			je.AppVersion = docker.ParseImageUrl(container.GetImage()).Tag
		}
	}
	return je
}

// taskDiag accumulates failure signals across an execution's task array so the
// failure banner can distinguish OOM (exit 137) from other failures.
type taskDiag struct {
	failedIndexes []int32
	oom           bool
	// exitCode is the exit code of the first failed task we encounter. It feeds
	// the failure banner so users see the actual process exit code, not just our
	// failure-mode label.
	exitCode *int32
}

func (s Statuser) listExecutionTasks(ctx context.Context, executionName string) ([]Task, taskDiag, error) {
	client, err := NewTasksClient(ctx, s.Infra.Deployer)
	if err != nil {
		return nil, taskDiag{}, fmt.Errorf("error initializing cloud run tasks client: %w", err)
	}
	defer client.Close()

	tasks := make([]Task, 0)
	diag := taskDiag{failedIndexes: make([]int32, 0)}
	it := client.ListTasks(ctx, &runpb.ListTasksRequest{Parent: executionName})
	for len(tasks) < maxTasksPerExecution {
		t, err := it.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, taskDiag{}, fmt.Errorf("error listing tasks: %w", err)
		}
		mapped := mapTask(t)
		tasks = append(tasks, mapped)
		if mapped.State == TaskStateFailed {
			diag.failedIndexes = append(diag.failedIndexes, mapped.Index)
			if res := t.GetLastAttemptResult(); res != nil {
				code := res.GetExitCode()
				if diag.exitCode == nil {
					diag.exitCode = &code
				}
				// Exit 137 (128 + SIGKILL) is the strongest OOM signal Cloud Run gives.
				if code == 137 {
					diag.oom = true
				}
			}
		}
	}
	return tasks, diag, nil
}

func mapTask(t *runpb.Task) Task {
	return Task{
		Index:    t.GetIndex(),
		State:    taskState(t),
		Attempts: t.GetRetried() + 1,
	}
}

func executionPhase(exec *runpb.Execution) JobExecutionPhase {
	if c := findCondition(exec.GetConditions(), "Completed"); c != nil {
		switch c.GetState() {
		case runpb.Condition_CONDITION_SUCCEEDED:
			return JobExecutionPhaseSucceeded
		case runpb.Condition_CONDITION_FAILED:
			return JobExecutionPhaseFailed
		}
	}
	if exec.GetCancelledCount() > 0 && exec.GetRunningCount() == 0 && exec.GetCompletionTime() != nil {
		return JobExecutionPhaseCancelled
	}
	if exec.GetCompletionTime() != nil {
		if exec.GetFailedCount() > 0 || exec.GetSucceededCount() < exec.GetTaskCount() {
			return JobExecutionPhaseFailed
		}
		return JobExecutionPhaseSucceeded
	}
	if exec.GetStartTime() != nil || exec.GetRunningCount() > 0 {
		return JobExecutionPhaseRunning
	}
	return JobExecutionPhaseQueued
}

func taskState(t *runpb.Task) TaskState {
	if t.GetCompletionTime() != nil {
		if res := t.GetLastAttemptResult(); res != nil && res.GetExitCode() == 0 {
			return TaskStateSucceeded
		}
		return TaskStateFailed
	}
	if t.GetStartTime() != nil {
		if t.GetRetried() > 0 {
			return TaskStateRetrying
		}
		return TaskStateRunning
	}
	return TaskStateQueued
}

// deriveExecutionFailure builds the failure banner for a failed execution.
// OOM is inferred from exit code 137 (SIGKILL) on a failed task — Cloud Run
// has no authoritative OOMKilled reason like Kubernetes does, so this is a
// best-effort heuristic. Other failures get a generic task-failure banner.
func deriveExecutionFailure(je JobExecution, diag taskDiag) *Failure {
	if je.Phase != JobExecutionPhaseFailed {
		return nil
	}
	count := len(diag.failedIndexes)
	if count == 0 {
		count = int(je.FailedCount)
	}
	if diag.oom {
		return &Failure{
			Code:     FailureOOMKilled,
			Title:    fmt.Sprintf("OOMKilled (%d task%s)", count, plural(count)),
			Message:  fmt.Sprintf("%d task%s exceeded the memory limit and exhausted retries (exit 137). Raise the memory limit or reduce per-task memory use.", count, plural(count)),
			ExitCode: diag.exitCode,
		}
	}
	return &Failure{
		Code:     FailureJobTaskFailed,
		Title:    fmt.Sprintf("Task failure (%d task%s)", count, plural(count)),
		Message:  fmt.Sprintf("%d of %d tasks failed and exhausted retries. Inspect the failed-task logs for the exit reason.", je.FailedCount, je.TaskCount),
		ExitCode: diag.exitCode,
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func findCondition(conditions []*runpb.Condition, conditionType string) *runpb.Condition {
	for _, c := range conditions {
		if c.GetType() == conditionType {
			return c
		}
	}
	return nil
}
