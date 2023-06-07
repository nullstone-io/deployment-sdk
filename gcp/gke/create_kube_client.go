package gke

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
)

func CreateKubeConfig(ctx context.Context, cluster k8s.ClusterInfoer, serviceAccount gcp.ServiceAccount) (*rest.Config, error) {
	configCreator := &k8s.ConfigCreator{
		ClusterInfoer: cluster,
		AuthInfoer:    ServiceAccountAuth{ServiceAccount: serviceAccount},
	}
	return configCreator.Create(ctx)
}

func CreateKubeClient(ctx context.Context, cluster k8s.ClusterInfoer, serviceAccount gcp.ServiceAccount) (*kubernetes.Clientset, error) {
	cfg, err := CreateKubeConfig(ctx, cluster, serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("error creating kube config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}
