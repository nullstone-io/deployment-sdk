package k8s

import (
	"context"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ClusterInfoer interface {
	ClusterInfo() (clientcmdapi.Cluster, error)
}

type AuthInfoer interface {
	AuthInfo(ctx context.Context) (clientcmdapi.AuthInfo, error)
}

// ConfigCreator constructs a kubernetes configuration from cluster information and auth information
type ConfigCreator struct {
	ClusterInfoer ClusterInfoer
	AuthInfoer    AuthInfoer
}

func (f *ConfigCreator) Create(ctx context.Context) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{}
	var err error
	if overrides.ClusterInfo, err = f.ClusterInfoer.ClusterInfo(); err != nil {
		return nil, err
	}
	if overrides.AuthInfo, err = f.AuthInfoer.AuthInfo(ctx); err != nil {
		return nil, err
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{}, overrides)
	return cc.ClientConfig()
}
