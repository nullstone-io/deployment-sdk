package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

const (
	ActionRestartDeployment = "restart-deployment"
	ActionKillTask          = "kill-task"
	ActionRerunJob          = "rerun-job"
)

type RestartDeploymentInput struct{}

type RestartDeploymentResult struct {
	Service     string    `json:"service"`
	RestartedAt time.Time `json:"restartedAt"`
}

type KillTaskInput struct {
	TaskArn string `json:"taskArn"`
	Reason  string `json:"reason,omitempty"`
}

type KillTaskResult struct {
	Task string `json:"task"`
}

type RerunJobInput struct {
	// TaskArn identifies a previously-launched task to clone. The new task inherits its task
	// definition, launch type / capacity provider, platform version, group, and (for awsvpc
	// tasks) subnets from this source task.
	TaskArn string `json:"taskArn"`
}

type RerunJobResult struct {
	// TaskArn is the ARN of the newly-launched task.
	TaskArn string `json:"taskArn"`
	// SourceTaskArn echoes the input task that was cloned.
	SourceTaskArn string `json:"sourceTaskArn"`
}

func NewActioner(ctx context.Context, source outputs.RetrieverSource, blockDetails workspace.Details) (workspace.Actioner, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, blockDetails.Workspace, blockDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}

	ws := blockDetails.Workspace
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.BlockId, ws.EnvId)
	outs.Deployer.RemoteProvider = credsFactory(types.AutomationPurposePerformAction, "deployer")

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
	case ActionRestartDeployment:
		return a.restartDeployment(ctx, options.Input)
	case ActionKillTask:
		return a.killTask(ctx, options.Input)
	case ActionRerunJob:
		return a.rerunJob(ctx, options.Input)
	default:
		return nil, workspace.ActionNotSupportedError{
			InnerErr: fmt.Errorf("unknown ecs action %q", options.Action),
		}
	}
}

func (a Actioner) newClient() *ecs.Client {
	return ecs.NewFromConfig(nsaws.NewConfig(a.Infra.Deployer, a.Infra.Region))
}

// restartDeployment forces a new deployment of the workspace's ECS service, keeping the
// current task definition. Mirrors `kubectl rollout restart deployment/<name>`.
func (a Actioner) restartDeployment(ctx context.Context, _ json.RawMessage) (*workspace.ActionResult, error) {
	if a.Infra.ServiceName == "" {
		return nil, fmt.Errorf("%s requires a service workspace", ActionRestartDeployment)
	}

	client := a.newClient()
	if _, err := client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(a.Infra.ClusterArn()),
		Service:            aws.String(a.Infra.ServiceName),
		ForceNewDeployment: true,
	}); err != nil {
		return nil, fmt.Errorf("error restarting service %q: %w", a.Infra.ServiceName, err)
	}

	now := time.Now().UTC()
	data, err := json.Marshal(RestartDeploymentResult{
		Service:     a.Infra.ServiceName,
		RestartedAt: now,
	})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "completed",
		Message: fmt.Sprintf("restarted service %q", a.Infra.ServiceName),
		Data:    data,
	}, nil
}

// killTask stops a single ECS task by ARN. The service controller (when present) will
// reconcile a replacement; standalone tasks will simply terminate.
func (a Actioner) killTask(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	var in KillTaskInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionKillTask, err)
		}
	}
	if in.TaskArn == "" {
		return nil, fmt.Errorf("%s requires taskArn", ActionKillTask)
	}
	reason := in.Reason
	if reason == "" {
		reason = "stopped from Nullstone"
	}

	client := a.newClient()
	if _, err := client.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(a.Infra.ClusterArn()),
		Task:    aws.String(in.TaskArn),
		Reason:  aws.String(reason),
	}); err != nil {
		return nil, fmt.Errorf("error stopping task %q: %w", in.TaskArn, err)
	}

	data, err := json.Marshal(KillTaskResult{Task: in.TaskArn})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "completed",
		Message: fmt.Sprintf("stopped task %q", in.TaskArn),
		Data:    data,
	}, nil
}

// rerunJob clones a previously-launched ECS task and runs a new copy of it. The source task ARN
// is required input; the new task inherits its task definition, launch type / capacity provider,
// platform version, group, and (for awsvpc tasks) subnets from the source.
//
// SecurityGroups aren't preserved on a task's attachments, so awsvpc tasks fall back to the VPC
// default security group. If non-default SGs are required, surface them via module outputs.
func (a Actioner) rerunJob(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	if a.Infra.ServiceName != "" {
		return nil, fmt.Errorf("%s is only supported on task workspaces", ActionRerunJob)
	}

	var in RerunJobInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionRerunJob, err)
		}
	}
	if in.TaskArn == "" {
		return nil, fmt.Errorf("%s requires taskArn (the source task to clone)", ActionRerunJob)
	}

	client := a.newClient()

	descOut, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(a.Infra.ClusterArn()),
		Tasks:   []string{in.TaskArn},
	})
	if err != nil {
		return nil, fmt.Errorf("error describing source task %q: %w", in.TaskArn, err)
	}
	if len(descOut.Tasks) == 0 {
		return nil, fmt.Errorf("source task %q not found", in.TaskArn)
	}
	src := descOut.Tasks[0]
	if aws.ToString(src.TaskDefinitionArn) == "" {
		return nil, fmt.Errorf("source task %q has no task definition arn", in.TaskArn)
	}

	startedBy := fmt.Sprintf("nullstone:rerun-job:%s", a.AppName)
	if len(startedBy) > 36 {
		startedBy = startedBy[:36]
	}

	runInput := &ecs.RunTaskInput{
		Cluster:              aws.String(a.Infra.ClusterArn()),
		TaskDefinition:       src.TaskDefinitionArn,
		Count:                aws.Int32(1),
		StartedBy:            aws.String(startedBy),
		EnableExecuteCommand: src.EnableExecuteCommand,
	}
	if pv := aws.ToString(src.PlatformVersion); pv != "" {
		runInput.PlatformVersion = aws.String(pv)
	}
	if g := aws.ToString(src.Group); g != "" {
		runInput.Group = aws.String(g)
	}

	// LaunchType and CapacityProviderStrategy are mutually exclusive in RunTask.
	if cp := aws.ToString(src.CapacityProviderName); cp != "" {
		runInput.CapacityProviderStrategy = []ecstypes.CapacityProviderStrategyItem{
			{CapacityProvider: aws.String(cp), Weight: 1},
		}
	} else if src.LaunchType != "" {
		runInput.LaunchType = src.LaunchType
	}

	if subnets := subnetsFromAttachments(src.Attachments); len(subnets) > 0 {
		runInput.NetworkConfiguration = &ecstypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
				Subnets: subnets,
			},
		}
	}

	out, err := client.RunTask(ctx, runInput)
	if err != nil {
		return nil, fmt.Errorf("error running task: %w", err)
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		return nil, fmt.Errorf("ecs RunTask failure: arn=%s reason=%s detail=%s",
			aws.ToString(f.Arn), aws.ToString(f.Reason), aws.ToString(f.Detail))
	}
	if len(out.Tasks) == 0 {
		return nil, fmt.Errorf("ecs RunTask returned no tasks and no failures")
	}

	newArn := aws.ToString(out.Tasks[0].TaskArn)
	data, err := json.Marshal(RerunJobResult{
		TaskArn:       newArn,
		SourceTaskArn: in.TaskArn,
	})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "started",
		Message: fmt.Sprintf("started task %q from %q", newArn, in.TaskArn),
		Data:    data,
	}, nil
}

func subnetsFromAttachments(atts []ecstypes.Attachment) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, att := range atts {
		if aws.ToString(att.Type) != "ElasticNetworkInterface" {
			continue
		}
		for _, kv := range att.Details {
			if aws.ToString(kv.Name) != "subnetId" {
				continue
			}
			v := aws.ToString(kv.Value)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}
