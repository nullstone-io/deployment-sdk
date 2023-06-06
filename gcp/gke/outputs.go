package gke

import (
	"encoding/base64"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/k8s"
	apimachineryschema "k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Outputs struct {
	ServiceNamespace  string             `ns:"service_namespace"`
	ServiceName       string             `ns:"service_name"`
	ImageRepoUrl      docker.ImageUrl    `ns:"image_repo_url,optional"`
	ImagePusher       gcp.ServiceAccount `ns:"image_pusher,optional"`
	Deployer          gcp.ServiceAccount `ns:"deployer"`
	MainContainerName string             `ns:"main_container_name,optional"`

	ClusterNamespace ClusterNamespaceOutputs `ns:",connectionContract:cluster-namespace/gcp/k8s:gke"`
}

type ClusterNamespaceOutputs struct {
	ClusterEndpoint      string `ns:"cluster_endpoint"`
	ClusterCACertificate string `ns:"cluster_ca_certificate"`
}

var _ k8s.ClusterInfoer = ClusterNamespaceOutputs{}

func (o ClusterNamespaceOutputs) ClusterInfo() (clientcmdapi.Cluster, error) {
	return getClusterInfo(o.ClusterEndpoint, o.ClusterCACertificate)
}

type ClusterOutputs struct {
	ClusterEndpoint      string `ns:"cluster_endpoint"`
	ClusterCACertificate string `ns:"cluster_ca_certificate"`
}

func (o ClusterOutputs) ClusterInfo() (clientcmdapi.Cluster, error) {
	return getClusterInfo(o.ClusterEndpoint, o.ClusterCACertificate)
}

func getClusterInfo(endpoint string, caCertificate string) (clientcmdapi.Cluster, error) {
	decodedCACert, err := base64.StdEncoding.DecodeString(caCertificate)
	if err != nil {
		return clientcmdapi.Cluster{}, fmt.Errorf("invalid cluster CA certificate: %w", err)
	}

	host, _, err := restclient.DefaultServerURL(endpoint, "", apimachineryschema.GroupVersion{Group: "", Version: "v1"}, true)
	if err != nil {
		return clientcmdapi.Cluster{}, fmt.Errorf("failed to parse GKE cluster host %q: %w", endpoint, err)
	}

	return clientcmdapi.Cluster{
		Server:                   host.String(),
		CertificateAuthorityData: decodedCACert,
	}, nil
}
