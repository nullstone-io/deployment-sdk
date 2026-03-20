package aks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/nullstone-io/deployment-sdk/azure"
	"github.com/nullstone-io/deployment-sdk/k8s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func CreateKubeConfig(ctx context.Context, cluster k8s.ClusterInfoer, principal azure.Principal) (*rest.Config, error) {
	configCreator := &k8s.ConfigCreator{
		ClusterInfoer: cluster,
		AuthInfoer:    PrincipalAuth{Principal: principal},
	}
	return configCreator.Create(ctx)
}

func CreateKubeClient(ctx context.Context, cluster k8s.ClusterInfoer, principal azure.Principal) (*kubernetes.Clientset, error) {
	cfg, err := CreateKubeConfig(ctx, cluster, principal)
	if err != nil {
		return nil, fmt.Errorf("error creating kube config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}

// AKSScopes is the default scope for Azure Kubernetes Service API access.
var AKSScopes = []string{"6dae42f8-4368-4678-94ff-3960e28e3630/.default"}

var _ k8s.AuthInfoer = PrincipalAuth{}

type PrincipalAuth struct {
	azure.Principal
}

func (p PrincipalAuth) AuthInfo(ctx context.Context) (clientcmdapi.AuthInfo, error) {
	token, err := p.GetToken(ctx, policy.TokenRequestOptions{Scopes: AKSScopes})
	if err != nil {
		return clientcmdapi.AuthInfo{}, fmt.Errorf("error retrieving kubernetes access token from Azure: %w", err)
	}
	return clientcmdapi.AuthInfo{
		Token: token.Token,
	}, nil
}
