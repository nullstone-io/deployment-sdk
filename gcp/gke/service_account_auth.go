package gke

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/k8s"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var GcpScopes = []string{
	"https://www.googleapis.com/auth/compute",
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/cloud-identity",
	"https://www.googleapis.com/auth/ndev.clouddns.readwrite",
	"https://www.googleapis.com/auth/devstorage.full_control",
	"https://www.googleapis.com/auth/userinfo.email",
}

var _ k8s.AuthInfoer = ServiceAccountAuth{}

type ServiceAccountAuth struct {
	gcp.ServiceAccount
}

func (s ServiceAccountAuth) AuthInfo(ctx context.Context) (clientcmdapi.AuthInfo, error) {
	kubeTokenSource, err := s.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return clientcmdapi.AuthInfo{}, err
	}
	token, err := kubeTokenSource.Token()
	if err != nil {
		return clientcmdapi.AuthInfo{}, fmt.Errorf("error retrieving kubernetes access token from google cloud: %w", err)
	}
	return clientcmdapi.AuthInfo{
		Token: token.AccessToken,
	}, nil
}
