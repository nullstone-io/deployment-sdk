package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nullstone-io/deployment-sdk/k8s/logs"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/workspace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	ActionRestartDeployment = "restart-deployment"
	ActionRerunJob          = "rerun-job"
	ActionKillPod           = "kill-pod"
)

type RestartDeploymentInput struct {
	DeploymentName string `json:"deploymentName"`
}

type RestartDeploymentResult struct {
	Deployment  string    `json:"deployment"`
	RestartedAt time.Time `json:"restartedAt"`
}

type RerunJobInput struct {
	JobDefinitionName string `json:"jobDefinitionName,omitempty"`
}

type RerunJobResult struct {
	Job string `json:"job"`
}

type KillPodInput struct {
	PodName            string `json:"podName"`
	GracePeriodSeconds *int64 `json:"gracePeriodSeconds,omitempty"`
}

type KillPodResult struct {
	Pod string `json:"pod"`
}

type Actioner struct {
	OsWriters         logging.OsWriters
	Namespace         string
	AppName           string
	JobDefinitionName string
	NewConfigFn       logs.NewConfiger
}

func (a Actioner) PerformAction(ctx context.Context, options workspace.ActionOptions) (*workspace.ActionResult, error) {
	switch options.Action {
	case ActionRestartDeployment:
		return a.restartDeployment(ctx, options.Input)
	case ActionRerunJob:
		return a.rerunJob(ctx, options.Input)
	case ActionKillPod:
		return a.killPod(ctx, options.Input)
	default:
		return nil, workspace.ActionNotSupportedError{
			InnerErr: fmt.Errorf("unknown k8s action %q", options.Action),
		}
	}
}

func (a Actioner) newClient(ctx context.Context) (*kubernetes.Clientset, error) {
	cfg, err := a.NewConfigFn(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes config: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating kube client: %w", err)
	}
	return client, nil
}

// restartDeployment patches the deployment's pod template with a kubectl.kubernetes.io/restartedAt
// annotation, matching the behavior of `kubectl rollout restart deployment/<name>`.
func (a Actioner) restartDeployment(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	var in RestartDeploymentInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionRestartDeployment, err)
		}
	}
	if in.DeploymentName == "" {
		return nil, fmt.Errorf("%s requires deploymentName", ActionRestartDeployment)
	}

	client, err := a.newClient(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		now.Format(time.RFC3339),
	)
	if _, err := client.AppsV1().Deployments(a.Namespace).Patch(
		ctx,
		in.DeploymentName,
		apitypes.StrategicMergePatchType,
		[]byte(patch),
		metav1.PatchOptions{},
	); err != nil {
		return nil, fmt.Errorf("error patching deployment %q: %w", in.DeploymentName, err)
	}

	data, err := json.Marshal(RestartDeploymentResult{
		Deployment:  in.DeploymentName,
		RestartedAt: now,
	})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "completed",
		Message: fmt.Sprintf("restarted deployment %q", in.DeploymentName),
		Data:    data,
	}, nil
}

// rerunJob creates a new k8s Job from the workspace's job template ConfigMap.
// The new Job is queued by the kube-scheduler; the call returns as soon as creation succeeds.
func (a Actioner) rerunJob(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	var in RerunJobInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionRerunJob, err)
		}
	}
	jobDefName := in.JobDefinitionName
	if jobDefName == "" {
		jobDefName = a.JobDefinitionName
	}
	if jobDefName == "" {
		return nil, fmt.Errorf("%s requires jobDefinitionName", ActionRerunJob)
	}

	client, err := a.newClient(ctx)
	if err != nil {
		return nil, err
	}

	jobDef, _, err := GetJobDefinition(ctx, client, a.Namespace, jobDefName)
	if err != nil {
		return nil, err
	}

	jobDef.Name = fmt.Sprintf("%s-%d", a.AppName, time.Now().Unix())
	jobDef.ResourceVersion = ""
	jobDef.UID = ""

	created, err := client.BatchV1().Jobs(a.Namespace).Create(ctx, jobDef, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creating job: %w", err)
	}

	data, err := json.Marshal(RerunJobResult{Job: created.Name})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "started",
		Message: fmt.Sprintf("created job %q from template %q", created.Name, jobDefName),
		Data:    data,
	}, nil
}

// killPod deletes a pod by name in the workspace's namespace.
// The replica set or job controller will reconcile a replacement.
func (a Actioner) killPod(ctx context.Context, input json.RawMessage) (*workspace.ActionResult, error) {
	var in KillPodInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, fmt.Errorf("invalid input for %s: %w", ActionKillPod, err)
		}
	}
	if in.PodName == "" {
		return nil, fmt.Errorf("%s requires podName", ActionKillPod)
	}

	client, err := a.newClient(ctx)
	if err != nil {
		return nil, err
	}

	delOpts := metav1.DeleteOptions{}
	if in.GracePeriodSeconds != nil {
		delOpts.GracePeriodSeconds = in.GracePeriodSeconds
	}
	if err := client.CoreV1().Pods(a.Namespace).Delete(ctx, in.PodName, delOpts); err != nil {
		return nil, fmt.Errorf("error deleting pod %q: %w", in.PodName, err)
	}

	data, err := json.Marshal(KillPodResult{Pod: in.PodName})
	if err != nil {
		return nil, err
	}
	return &workspace.ActionResult{
		Status:  "completed",
		Message: fmt.Sprintf("deleted pod %q", in.PodName),
		Data:    data,
	}, nil
}
