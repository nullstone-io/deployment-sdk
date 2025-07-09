package ecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"time"
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
	Id                 string    `json:"id"`
	AppVersion         string    `json:"appVersion"`
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

	svc, err := GetService(ctx, s.Infra)
	if err != nil {
		return st, err
	}
	lbs := StatusLoadBalancersFromEcsService(svc)
	if err := lbs.RefreshHealth(ctx, s.Infra); err != nil {
		return st, err
	}

	taskDefs := TaskDefinitionsCache{}
	for _, task := range tasks {
		taskDef, err := taskDefs.Get(ctx, s.Infra, task.TaskDefinitionArn)
		if err != nil {
			return st, err
		}
		statusTask := StatusTaskFromEcsTask(task)
		statusTask.Enrich(lbs, taskDef)
		st.Tasks = append(st.Tasks, statusTask)
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
