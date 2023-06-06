package gke

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func CreateKubeClient(ctx context.Context, serviceAccount gcp.ServiceAccount, cluster k8s.ClusterInfoer) (*kubernetes.Clientset, error) {
	configCreator := &k8s.ConfigCreator{
		ClusterInfoer: cluster,
		AuthInfoer:    ServiceAccountAuth{ServiceAccount: serviceAccount},
	}
	cfg, err := configCreator.Create(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating kube config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}
