package eks

import (
	"encoding/base64"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/k8s"
	apimachineryschema "k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Outputs struct {
	ServiceNamespace  string          `ns:"service_namespace"`
	ServiceName       string          `ns:"service_name"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	ImagePusher       nsaws.User      `ns:"image_pusher,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	JobDefinitionName string          `ns:"job_definition_name,optional"`

	Region           string                  `ns:"region"`
	ClusterNamespace ClusterNamespaceOutputs `ns:",connectionContract:cluster-namespace/aws/kubernetes:eks,optional"`
}

func (o Outputs) ClusterArn() string {
	return o.ClusterNamespace.ClusterArn
}

type ClusterInfoer interface {
	k8s.ClusterInfoer
	GetClusterName() string
}

var _ ClusterInfoer = ClusterNamespaceOutputs{}

type ClusterNamespaceOutputs struct {
	ClusterName          string `ns:"cluster_name"`
	ClusterArn           string `ns:"cluster_arn"`
	ClusterEndpoint      string `ns:"cluster_endpoint"`
	ClusterCACertificate string `ns:"cluster_ca_certificate"`
}

func (o ClusterNamespaceOutputs) GetClusterName() string {
	return o.ClusterName
}

func (o ClusterNamespaceOutputs) ClusterInfo() (clientcmdapi.Cluster, error) {
	return GetClusterInfo(o.ClusterEndpoint, o.ClusterCACertificate)
}

func GetClusterInfo(endpoint, caCertificate string) (clientcmdapi.Cluster, error) {
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
