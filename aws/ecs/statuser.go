package ecs

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

var (
	_ app.StatusOverviewResult = StatusOverview{}
)

type StatusOverview struct {
	Deployments []StatusOverviewDeployment `json:"deployments"`
}

func (s StatusOverview) GetDeploymentVersions() []string {
	refs := make([]string, 0)
	for _, d := range s.Deployments {
		refs = append(refs, d.AppVersion)
	}
	return refs
}

type StatusOverviewDeployment struct {
	Id                     string    `json:"id"`
	AppVersion             string    `json:"appVersion"`
	CreatedAt              time.Time `json:"createdAt"`
	Status                 string    `json:"status"`
	RolloutState           string    `json:"rolloutState"`
	RolloutStateReason     string    `json:"rolloutStateReason"`
	DesiredCount           int32     `json:"desiredCount"`
	PendingCount           int32     `json:"pendingCount"`
	RunningCount           int32     `json:"runningCount"`
	FailedCount            int32     `json:"failedCount"`
	TaskDefinitionFamily   string    `json:"taskDefinitionFamily"`
	TaskDefinitionRevision int32     `json:"taskDefinitionRevision"`
}

// ClusterInfo identifies the AWS cluster an ECS app is running on.
type ClusterInfo struct {
	Region      string `json:"region"`
	ClusterName string `json:"clusterName"`
}

type Status struct {
	Cluster     ClusterInfo `json:"cluster"`
	ServiceName string      `json:"serviceName"`
	// IsJob is true when the workspace is a one-off RunTask workspace (no ECS service).
	// Frontend uses this to route to the job-executions view rather than the deployments view.
	IsJob      bool              `json:"isJob"`
	Tasks      []StatusTask      `json:"tasks"`
	Executions []EcsJobExecution `json:"executions"`
}

func NewStatuser(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Statuser, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

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

func (s Statuser) StatusOverview(ctx context.Context) (app.StatusOverviewResult, error) {
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

	tagsCache := &ResourceTagsCache{Infra: s.Infra}
	for _, deployment := range svc.Deployments {
		appVersion, err := tagsCache.Get(ctx, *deployment.TaskDefinition, VersionTagKey)
		if err != nil {
			// NOTE: We're silently ignoring retrieval of app version
		}

		family, revision := parseTaskDefinition(aws.ToString(deployment.TaskDefinition))
		so.Deployments = append(so.Deployments, StatusOverviewDeployment{
			Id:                     aws.ToString(deployment.Id),
			AppVersion:             appVersion,
			CreatedAt:              aws.ToTime(deployment.CreatedAt),
			Status:                 aws.ToString(deployment.Status),
			RolloutState:           string(deployment.RolloutState),
			RolloutStateReason:     aws.ToString(deployment.RolloutStateReason),
			DesiredCount:           deployment.DesiredCount,
			PendingCount:           deployment.PendingCount,
			RunningCount:           deployment.RunningCount,
			FailedCount:            deployment.FailedTasks,
			TaskDefinitionFamily:   family,
			TaskDefinitionRevision: revision,
		})
	}
	return so, nil
}

func (s Statuser) Status(ctx context.Context) (any, error) {
	isJob := s.Infra.ServiceName == ""
	st := Status{
		Cluster: ClusterInfo{
			Region:      s.Infra.Region,
			ClusterName: parseClusterName(s.Infra.ClusterArn()),
		},
		IsJob:      isJob,
		Tasks:      make([]StatusTask, 0),
		Executions: make([]EcsJobExecution, 0),
	}
	tasks, err := s.getTasks(ctx)
	if err != nil {
		return st, err
	}

	svc, err := GetService(ctx, s.Infra)
	if err != nil {
		return st, err
	}
	if svc != nil {
		st.ServiceName = aws.ToString(svc.ServiceName)
	} else {
		st.ServiceName = s.Infra.ServiceName
	}
	lbs := StatusLoadBalancersFromEcsService(svc)
	if err := lbs.RefreshHealth(ctx, s.Infra); err != nil {
		return st, err
	}

	taskDefs := TaskDefinitionsCache{}
	// Job workspaces resolve appVersion per-execution from the task definition's
	// nullstone.io/version tag. Service workspaces don't need this — appVersion is
	// already attached to the service deployment in StatusOverview.
	var tagsCache *ResourceTagsCache
	if isJob {
		tagsCache = &ResourceTagsCache{Infra: s.Infra}
	}
	for _, task := range tasks {
		taskDef, err := taskDefs.Get(ctx, s.Infra, task.TaskDefinitionArn)
		if err != nil {
			return st, err
		}
		statusTask := StatusTaskFromEcsTask(task)
		statusTask.Enrich(lbs, taskDef)
		st.Tasks = append(st.Tasks, statusTask)

		if isJob {
			// Silently swallow tag-lookup errors: a missing or unreadable tag should
			// not break the executions list. The card simply omits the commit link.
			appVersion, _ := tagsCache.Get(ctx, aws.ToString(task.TaskDefinitionArn), VersionTagKey)
			st.Executions = append(st.Executions, EcsJobExecutionFromStatusTask(statusTask, appVersion))
		}
	}

	if isJob {
		// ECS forgets stopped tasks after about an hour. The cluster's task state-change log fills in
		// the runs that DescribeTasks no longer returns; live tasks win when both know about a task.
		history, err := GetTaskFamilyHistory(ctx, s.Infra, time.Now())
		if err != nil {
			// History is best-effort: the live list is still accurate for the last hour.
			if s.OsWriters != nil {
				fmt.Fprintf(s.OsWriters.Stderr(), "warning: unable to read task execution history: %s\n", err)
			}
		}
		live := map[string]bool{}
		for _, task := range tasks {
			live[aws.ToString(task.TaskArn)] = true
		}
		for _, task := range history {
			if live[aws.ToString(task.TaskArn)] {
				continue
			}
			// An older revision may be deregistered; enrichment is cosmetic, so a lookup failure is not fatal.
			taskDef, _ := taskDefs.Get(ctx, s.Infra, task.TaskDefinitionArn)
			statusTask := StatusTaskFromEcsTask(task)
			statusTask.Enrich(lbs, taskDef)
			appVersion, _ := tagsCache.Get(ctx, aws.ToString(task.TaskDefinitionArn), VersionTagKey)
			st.Executions = append(st.Executions, EcsJobExecutionFromStatusTask(statusTask, appVersion))
		}
		st.Executions = newestExecutions(st.Executions, maxExecutions)
	}

	return st, nil
}

// maxExecutions bounds the status payload; Razor sorts and renders whatever it receives.
const maxExecutions = 100

// newestExecutions keeps the n most recent executions: running ones first, then by stop time.
func newestExecutions(executions []EcsJobExecution, n int) []EcsJobExecution {
	ts := func(e EcsJobExecution) time.Time {
		if e.StoppedAt != nil {
			return *e.StoppedAt
		}
		if e.StartedAt != nil {
			return *e.StartedAt
		}
		return time.Time{}
	}
	sort.SliceStable(executions, func(i, j int) bool {
		a, b := executions[i], executions[j]
		if (a.StoppedAt == nil) != (b.StoppedAt == nil) {
			return a.StoppedAt == nil
		}
		return ts(a).After(ts(b))
	})
	if len(executions) > n {
		return executions[:n]
	}
	return executions
}

func (s Statuser) getTasks(ctx context.Context) ([]ecstypes.Task, error) {
	if s.Infra.ServiceName == "" {
		return GetTaskFamilyTasks(ctx, s.Infra)
	} else {
		return GetServiceTasks(ctx, s.Infra)
	}
}
