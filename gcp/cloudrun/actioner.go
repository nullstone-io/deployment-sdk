package cloudrun

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/nullstone-io/deployment-sdk/gcp/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

const (
	ActionRestartRevision = "restart-revision"
	ActionRouteTraffic    = "route-traffic"
	ActionCancelExecution = "cancel-execution"
	ActionRerunJob        = "rerun-job"

	// restartAnnotation is bumped on the revision template to force Cloud Run to
	// mint a fresh revision on UpdateService even when nothing else changed.
	restartAnnotation = "nullstone.io/restarted-at"
)

type RestartRevisionInput struct {
	// RevisionName is the revision the restart was requested from. Cloud Run
	// revisions are immutable, so the restart redeploys the service's current
	// template as a new revision; this value is echoed back for traceability.
	RevisionName string `json:"revisionName"`
}

type RestartRevisionResult struct {
	Service      string    `json:"service"`
	FromRevision string    `json:"fromRevision,omitempty"`
	RestartedAt  time.Time `json:"restartedAt"`
	Operation    string    `json:"operation,omitempty"`
}

type RouteTrafficInput struct {
	RevisionName string `json:"revisionName"`
	Percent      int32  `json:"percent"`
}

type RouteTrafficResult struct {
	Service      string `json:"service"`
	RevisionName string `json:"revisionName"`
	Percent      int32  `json:"percent"`
}

type CancelExecutionInput struct {
	ExecutionId string `json:"executionId"`
}

type CancelExecutionResult struct {
	Execution string `json:"execution"`
}

type RerunJobInput struct {
	// ExecutionId is the execution the rerun was requested from. It is echoed
	// back but not used to launch: Cloud Run always runs a fresh execution.
	ExecutionId string `json:"executionId"`
	// OnlyFailedTasks is accepted for API parity with other providers, but Cloud
	// Run Jobs cannot rerun individual task indexes — see rerunJob.
	OnlyFailedTasks bool `json:"onlyFailedTasks"`
}

type RerunJobResult struct {
	Job             string `json:"job"`
	SourceExecution string `json:"sourceExecution,omitempty"`
	Operation       string `json:"operation,omitempty"`
}

func NewActioner(ctx context.Context, source outputs.RetrieverSource, blockDetails workspace.Details) (workspace.Actioner, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, blockDetails.Workspace, blockDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}

	ws := blockDetails.Workspace
	outs.Deployer.RemoteTokenSourcer = creds.NewTokenSourcer(source, ws.StackId, ws.BlockId, ws.EnvId, types.AutomationPurposePerformAction, "deployer")

	return Actioner{
		Infra:   outs,
		AppName: blockDetails.Block.Name,
	}, nil
}

type Actioner struct {
	Infra   Outputs
	AppName string
}

func (a Actioner) PerformAction(ctx context.Context, options workspace.ActionOptions) (*workspace.ActionResult, error) {
	switch options.Action {
	case ActionRestartRevision:
		return a.restartRevision(ctx, options.Input)
	case ActionRouteTraffic:
		return a.routeTraffic(ctx, options.Input)
	case ActionCancelExecution:
		return a.cancelExecution(ctx, options.Input)
	case ActionRerunJob:
		return a.rerunJob(ctx, options.Input)
	default:
		return nil, workspace.ActionNotSupportedError{
			InnerErr: fmt.Errorf("unknown cloud run action %q", options.Action),
		}
	}
}

// restartRevision forces a new revision of the service from its current
// template, draining the old instances. Cloud Run revisions are immutable, so
// there's no in-place restart; this mirrors `kubectl rollout restart`.
//
// Traffic routing is left untouched: if the service routes to LATEST, the new
// revision serves automatically; if traffic is pinned to a specific revision,
// the new revision is created ready-but-idle and can be promoted with
// route-traffic.
func (a Actioner) restartRevision(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	if a.Infra.ServiceId == "" {
		return nil, fmt.Errorf("%s requires a service workspace", ActionRestartRevision)
	}
	var in RestartRevisionInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionRestartRevision, err)
		}
	}

	client, err := NewServicesClient(ctx, a.Infra.Deployer)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloud run services client: %w", err)
	}
	defer client.Close()

	svc, err := client.GetService(ctx, &runpb.GetServiceRequest{Name: a.Infra.ServiceId})
	if err != nil {
		return nil, fmt.Errorf("error retrieving service: %w", err)
	}
	tmpl := svc.GetTemplate()
	if tmpl == nil {
		return nil, fmt.Errorf("service %q has no template to restart", a.Infra.ServiceId)
	}

	// Clear the pinned revision name so the server auto-generates a fresh one,
	// and bump the restart annotation so the template differs from the current
	// revision (otherwise Cloud Run treats the update as a no-op).
	tmpl.Revision = ""
	if tmpl.Annotations == nil {
		tmpl.Annotations = map[string]string{}
	}
	restartedAt := time.Now().UTC()
	tmpl.Annotations[restartAnnotation] = restartedAt.Format(time.RFC3339Nano)

	op, err := client.UpdateService(ctx, &runpb.UpdateServiceRequest{Service: svc})
	if err != nil {
		return nil, fmt.Errorf("error restarting service %q: %w", a.Infra.ServiceId, err)
	}
	// Don't block on the new revision becoming ready; that can take a while. The
	// status view reflects progress on the next poll.

	data, err := json.Marshal(RestartRevisionResult{
		Service:      a.Infra.ServiceName(),
		FromRevision: in.RevisionName,
		RestartedAt:  restartedAt,
		Operation:    op.Name(),
	})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "started",
		Message: fmt.Sprintf("restart triggered for service %q", a.Infra.ServiceId),
		Data:    data,
	}, nil
}

// routeTraffic points `percent` of the service's traffic at revisionName. When
// percent is less than 100 the remainder is routed to the latest ready revision.
func (a Actioner) routeTraffic(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	if a.Infra.ServiceId == "" {
		return nil, fmt.Errorf("%s requires a service workspace", ActionRouteTraffic)
	}
	var in RouteTrafficInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionRouteTraffic, err)
		}
	}
	if in.RevisionName == "" {
		return nil, fmt.Errorf("%s requires revisionName", ActionRouteTraffic)
	}
	if in.Percent <= 0 || in.Percent > 100 {
		return nil, fmt.Errorf("%s requires percent in 1..100, got %d", ActionRouteTraffic, in.Percent)
	}

	client, err := NewServicesClient(ctx, a.Infra.Deployer)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloud run services client: %w", err)
	}
	defer client.Close()

	svc, err := client.GetService(ctx, &runpb.GetServiceRequest{Name: a.Infra.ServiceId})
	if err != nil {
		return nil, fmt.Errorf("error retrieving service: %w", err)
	}
	svc.Traffic = buildTrafficTargets(in.RevisionName, in.Percent, shortName(svc.GetLatestReadyRevision()))

	op, err := client.UpdateService(ctx, &runpb.UpdateServiceRequest{Service: svc})
	if err != nil {
		return nil, fmt.Errorf("error routing traffic to %q: %w", in.RevisionName, err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return nil, fmt.Errorf("error waiting for traffic update: %w", err)
	}

	data, err := json.Marshal(RouteTrafficResult{
		Service:      a.Infra.ServiceName(),
		RevisionName: in.RevisionName,
		Percent:      in.Percent,
	})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "completed",
		Message: fmt.Sprintf("routed %d%% traffic to %q", in.Percent, in.RevisionName),
		Data:    data,
	}, nil
}

// buildTrafficTargets builds a traffic split that sends `percent` to
// revisionName. When percent < 100 the remainder goes to the latest ready
// revision, or to the LATEST allocation when that's unknown or is the same
// revision.
func buildTrafficTargets(revisionName string, percent int32, latestReady string) []*runpb.TrafficTarget {
	targets := []*runpb.TrafficTarget{{
		Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
		Revision: revisionName,
		Percent:  percent,
	}}
	if percent >= 100 {
		return targets
	}

	remainder := 100 - percent
	if latestReady != "" && latestReady != revisionName {
		targets = append(targets, &runpb.TrafficTarget{
			Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
			Revision: latestReady,
			Percent:  remainder,
		})
	} else {
		targets = append(targets, &runpb.TrafficTarget{
			Type:    runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST,
			Percent: remainder,
		})
	}
	return targets
}

// cancelExecution cancels a running job execution. It waits for the cancel to
// be acknowledged so the caller gets a real confirmation (or the failure).
func (a Actioner) cancelExecution(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	if a.Infra.JobId == "" {
		return nil, fmt.Errorf("%s requires a job workspace", ActionCancelExecution)
	}
	var in CancelExecutionInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionCancelExecution, err)
		}
	}
	if in.ExecutionId == "" {
		return nil, fmt.Errorf("%s requires executionId", ActionCancelExecution)
	}

	client, err := NewExecutionsClient(ctx, a.Infra.Deployer)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloud run executions client: %w", err)
	}
	defer client.Close()

	op, err := client.CancelExecution(ctx, &runpb.CancelExecutionRequest{Name: a.executionName(in.ExecutionId)})
	if err != nil {
		return nil, fmt.Errorf("error cancelling execution %q: %w", in.ExecutionId, err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return nil, fmt.Errorf("error waiting for execution %q to cancel: %w", in.ExecutionId, err)
	}

	data, err := json.Marshal(CancelExecutionResult{Execution: in.ExecutionId})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "completed",
		Message: fmt.Sprintf("cancelled execution %q", in.ExecutionId),
		Data:    data,
	}, nil
}

// rerunJob launches a fresh execution of the job. Cloud Run Jobs has no API to
// rerun individual task indexes, so OnlyFailedTasks cannot be honored — the
// whole job is re-executed and the result message says so. (The job's own
// per-task retry policy handles transient task failures within an execution.)
func (a Actioner) rerunJob(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	if a.Infra.JobId == "" {
		return nil, fmt.Errorf("%s requires a job workspace", ActionRerunJob)
	}
	var in RerunJobInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionRerunJob, err)
		}
	}

	client, err := NewJobsClient(ctx, a.Infra.Deployer)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloud run jobs client: %w", err)
	}
	defer client.Close()

	op, err := client.RunJob(ctx, &runpb.RunJobRequest{Name: a.Infra.JobId})
	if err != nil {
		return nil, fmt.Errorf("error running job %q: %w", a.Infra.JobId, err)
	}
	// Don't wait: the execution runs asynchronously and may take a long time.

	data, err := json.Marshal(RerunJobResult{
		Job:             a.Infra.JobName(),
		SourceExecution: in.ExecutionId,
		Operation:       op.Name(),
	})
	if err != nil {
		return nil, err
	}

	msg := fmt.Sprintf("started a new execution of job %q", a.Infra.JobId)
	if in.OnlyFailedTasks {
		msg = fmt.Sprintf("Cloud Run reruns the entire job; individual failed tasks can't be targeted. Started a full execution of job %q.", a.Infra.JobId)
	}
	return &workspace.ActionResult{
		Status:  "started",
		Message: msg,
		Data:    data,
	}, nil
}

func (a Actioner) executionName(executionId string) string {
	return fmt.Sprintf("%s/executions/%s", a.Infra.JobId, executionId)
}
