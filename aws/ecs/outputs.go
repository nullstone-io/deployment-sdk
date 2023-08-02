package ecs

import (
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/docker"
)

type Outputs struct {
	Region            string          `ns:"region"`
	ServiceName       string          `ns:"service_name"`
	TaskArn           string          `ns:"task_arn"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher       nsaws.User      `ns:"image_pusher,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`

	Cluster          ClusterOutputs          `ns:",connectionContract:cluster/aws/ecs:*,optional"`
	ClusterNamespace ClusterNamespaceOutputs `ns:",connectionContract:cluster-namespace/aws/ecs:*,optional"`
}

func (o Outputs) ClusterArn() string {
	if o.ClusterNamespace.ClusterArn != "" {
		return o.ClusterNamespace.ClusterArn
	}
	return o.Cluster.ClusterArn
}

func (o Outputs) TaskFamily() string {
	return o.TaskArn[:len(o.TaskArn)-len(":1")]
}

type ClusterNamespaceOutputs struct {
	ClusterArn string `ns:"cluster_arn"`
}

type ClusterOutputs struct {
	ClusterArn string `ns:"cluster_arn"`
}
