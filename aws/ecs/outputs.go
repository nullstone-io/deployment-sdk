package ecs

import (
	"strings"

	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	Region            string            `ns:"region"`
	ServiceName       string            `ns:"service_name"`
	TaskArn           string            `ns:"task_arn"`
	ImageRepoUrl      docker.ImageUrl   `ns:"image_repo_url,optional"`
	MainContainerName string            `ns:"main_container_name,optional"`
	Deployer          nsaws.IamIdentity `ns:"deployer,optional"`

	Cluster          ClusterOutputs          `ns:",connectionContract:cluster/aws/ecs:*,optional"`
	ClusterNamespace ClusterNamespaceOutputs `ns:",connectionContract:cluster-namespace/aws/ecs:*,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.BlockId, ws.EnvId)
	o.Deployer.RemoteProvider = credsFactory(types.AutomationPurposeDeploy, "deployer")
}

// ClusterArn has the following format: arn:aws:ecs:<region>:<account-id>:cluster/<cluster-name>
func (o *Outputs) ClusterArn() string {
	if o.ClusterNamespace.ClusterArn != "" {
		return o.ClusterNamespace.ClusterArn
	}
	return o.Cluster.ClusterArn
}

// ExecutionsLogGroupName is the CloudWatch log group where the cluster records ECS task state-change events.
// It is empty when the cluster module predates execution history, in which case only live tasks are visible.
func (o *Outputs) ExecutionsLogGroupName() string {
	if o.ClusterNamespace.ExecutionsLogGroupName != "" {
		return o.ClusterNamespace.ExecutionsLogGroupName
	}
	return o.Cluster.ExecutionsLogGroupName
}

func (o *Outputs) TaskFamily() string {
	temp := strings.Split(o.TaskArn, ":")
	family := temp[len(temp)-2]
	return strings.Split(family, "/")[1]
}

type ClusterNamespaceOutputs struct {
	ClusterArn             string `ns:"cluster_arn"`
	ExecutionsLogGroupName string `ns:"executions_log_group_name,optional"`
}

type ClusterOutputs struct {
	ClusterArn             string `ns:"cluster_arn"`
	ExecutionsLogGroupName string `ns:"executions_log_group_name,optional"`
}
