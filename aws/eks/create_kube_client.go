package eks

import (
	"context"
	"fmt"

	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ClusterMeta interface {
	k8s.ClusterInfoer
	AwsContext() ClusterAwsContext
}

type ClusterAwsContext struct {
	Region    string
	ClusterId string
}

func CreateKubeConfig(ctx context.Context, cluster ClusterMeta, user nsaws.User) (*rest.Config, error) {
	awsContext := cluster.AwsContext()
	configCreator := &k8s.ConfigCreator{
		ClusterInfoer: cluster,
		AuthInfoer: IamUserAuth{
			User:      user,
			Region:    awsContext.Region,
			ClusterId: awsContext.ClusterId,
		},
	}
	return configCreator.Create(ctx)
}

func CreateKubeClient(ctx context.Context, cluster ClusterMeta, user nsaws.User) (*kubernetes.Clientset, error) {
	cfg, err := CreateKubeConfig(ctx, cluster, user)
	if err != nil {
		return nil, fmt.Errorf("error creating kube config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}
