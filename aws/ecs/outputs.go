package ecs

import (
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/docker"
	"strings"
)

type Outputs struct {
	Region            string          `ns:"region"`
	ServiceName       string          `ns:"service_name"`
	TaskArn           string          `ns:"task_arn"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher       nsaws.User      `ns:"image_pusher,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`

	LogReader nsaws.User `ns:"log_reader,optional"`

	Cluster          ClusterOutputs          `ns:",connectionContract:cluster/aws/ecs:*,optional"`
	ClusterNamespace ClusterNamespaceOutputs `ns:",connectionContract:cluster-namespace/aws/ecs:*,optional"`
}

// ClusterArn has the following format: arn:aws:ecs:<region>:<account-id>:cluster/<cluster-name>
func (o Outputs) ClusterArn() string {
	if o.ClusterNamespace.ClusterArn != "" {
		return o.ClusterNamespace.ClusterArn
	}
	return o.Cluster.ClusterArn
}

func (o Outputs) ClusterName() string {
	arn := o.ClusterArn()
	_, clusterName, found := strings.Cut(arn, "cluster/")
	if !found {
		return ""
	}
	return clusterName
}

func (o Outputs) AccountId() string {
	arn := o.ClusterArn()
	tokens := strings.Split(arn, ":")
	if len(tokens) < 5 {
		return ""
	}
	return tokens[4]
}

func (o Outputs) TaskFamily() string {
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
