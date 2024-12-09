package ecs

import (
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"strings"
)

type Outputs struct {
	Region            string          `ns:"region"`
	ServiceName       string          `ns:"service_name"`
	TaskArn           string          `ns:"task_arn"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`

	Cluster          ClusterOutputs          `ns:",connectionContract:cluster/aws/ecs:*,optional"`
	ClusterNamespace ClusterNamespaceOutputs `ns:",connectionContract:cluster-namespace/aws/ecs:*,optional"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.Uid)
	o.Deployer.RemoteProvider = credsFactory("deployer")
}

// ClusterArn has the following format: arn:aws:ecs:<region>:<account-id>:cluster/<cluster-name>
func (o *Outputs) ClusterArn() string {
	if o.ClusterNamespace.ClusterArn != "" {
		return o.ClusterNamespace.ClusterArn
	}
	return o.Cluster.ClusterArn
}

func (o *Outputs) TaskFamily() string {
	temp := strings.Split(o.TaskArn, ":")
	family := temp[len(temp)-2]
	return strings.Split(family, "/")[1]
}

type ClusterNamespaceOutputs struct {
	ClusterArn string `ns:"cluster_arn"`
}

type ClusterOutputs struct {
	ClusterArn string `ns:"cluster_arn"`
}
