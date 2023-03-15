package gke

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var GcpScopes = []string{
	"https://www.googleapis.com/auth/compute",
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/cloud-identity",
	"https://www.googleapis.com/auth/ndev.clouddns.readwrite",
	"https://www.googleapis.com/auth/devstorage.full_control",
	"https://www.googleapis.com/auth/userinfo.email",
}

func CreateKubeClient(ctx context.Context, serviceAccount gcp.ServiceAccount, cluster ClusterOutputs) (*kubernetes.Clientset, error) {
	configCreator := &k8s.ConfigCreator{
		TokenSourcer:  serviceAccount,
		ClusterInfoer: cluster,
	}
	cfg, err := configCreator.Create(ctx, GcpScopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating kube config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}
