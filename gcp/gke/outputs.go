package gke

import (
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/k8s"
)

type Outputs struct {
	ServiceNamespace  string             `ns:"service_namespace"`
	ServiceName       string             `ns:"service_name"`
	ImageRepoUrl      docker.ImageUrl    `ns:"image_repo_url,optional"`
	ImagePusher       gcp.ServiceAccount `ns:"image_pusher,optional"`
	MainContainerName string             `ns:"main_container_name,optional"`

	Cluster ClusterOutputs `ns:",connectionContract:cluster/gcp/k8s:gke"`
}

type ClusterOutputs struct {
	ClusterId            string             `ns:"cluster_id"`
	ClusterEndpoint      string             `ns:"cluster_endpoint"`
	ClusterCACertificate string             `ns:"cluster_ca_certificate"`
	Deployer             gcp.ServiceAccount `ns:"deployer"`
}

func (o ClusterOutputs) ClusterInfo() k8s.ClusterInfo {
	return k8s.ClusterInfo{
		ID:            o.ClusterId,
		Endpoint:      o.ClusterEndpoint,
		CACertificate: o.ClusterCACertificate,
	}
}
