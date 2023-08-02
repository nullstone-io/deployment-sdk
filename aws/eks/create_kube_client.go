package eks

import (
	"context"
	"fmt"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func CreateKubeConfig(ctx context.Context, region string, cluster ClusterInfoer, user nsaws.User) (*rest.Config, error) {
	configCreator := &k8s.ConfigCreator{
		ClusterInfoer: cluster,
		AuthInfoer: IamUserAuth{
			User:      user,
			Region:    region,
			ClusterId: cluster.GetClusterName(),
		},
	}
	return configCreator.Create(ctx)
}

func CreateKubeClient(ctx context.Context, region string, cluster ClusterInfoer, user nsaws.User) (*kubernetes.Clientset, error) {
	cfg, err := CreateKubeConfig(ctx, region, cluster, user)
	if err != nil {
		return nil, fmt.Errorf("error creating kube config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}
