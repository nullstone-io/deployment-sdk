package eks

import (
	"encoding/base64"
	"fmt"

	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	apimachineryschema "k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Outputs struct {
	ServiceNamespace  string          `ns:"service_namespace"`
	ServiceName       string          `ns:"service_name"`
	ImageRepoUrl      docker.ImageUrl `ns:"image_repo_url,optional"`
	Deployer          nsaws.User      `ns:"deployer,optional"`
	MainContainerName string          `ns:"main_container_name,optional"`
	JobDefinitionName string          `ns:"job_definition_name,optional"`

	ClusterNamespace ClusterNamespaceOutputs `ns:",connectionContract:cluster-namespace/aws/k8s:eks"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.Uid)
	o.Deployer.RemoteProvider = credsFactory("deployer")
}

type ClusterNamespaceOutputs struct {
	Region               string `ns:"region"`
	ClusterId            string `ns:"cluster_id"`
	ClusterEndpoint      string `ns:"cluster_endpoint"`
	ClusterCACertificate string `ns:"cluster_ca_certificate"`
}

var _ ClusterMeta = ClusterNamespaceOutputs{}

func (o ClusterNamespaceOutputs) ClusterInfo() (clientcmdapi.Cluster, error) {
	return GetClusterInfo(o.ClusterEndpoint, o.ClusterCACertificate)
}

func (o ClusterNamespaceOutputs) AwsContext() ClusterAwsContext {
	return ClusterAwsContext{
		Region:    o.Region,
		ClusterId: o.ClusterId,
	}
}

func GetClusterInfo(endpoint string, caCertificate string) (clientcmdapi.Cluster, error) {
	decodedCACert, err := base64.StdEncoding.DecodeString(caCertificate)
	if err != nil {
		return clientcmdapi.Cluster{}, fmt.Errorf("invalid cluster CA certificate: %w", err)
	}

	host, _, err := restclient.DefaultServerURL(endpoint, "", apimachineryschema.GroupVersion{Group: "", Version: "v1"}, true)
	if err != nil {
		return clientcmdapi.Cluster{}, fmt.Errorf("failed to parse EKS cluster host %q: %w", endpoint, err)
	}

	return clientcmdapi.Cluster{
		Server:                   host.String(),
		CertificateAuthorityData: decodedCACert,
	}, nil
}
