package app

import (
	"context"
	"github.com/nullstone-io/deployment-sdk/contract"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Providers map[types.ModuleContractName]Provider

func (s Providers) FindPusher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Pusher, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil || factory.NewPusher == nil {
		return nil, nil
	}
	return factory.NewPusher(ctx, osWriters, source, appDetails)
}

func (s Providers) FindDeployer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Deployer, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil || factory.NewDeployer == nil {
		return nil, nil
	}
	return factory.NewDeployer(ctx, osWriters, source, appDetails)
}

func (s Providers) FindDeployWatcher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (DeployWatcher, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil || factory.NewDeployWatcher == nil {
		return nil, nil
	}
	return factory.NewDeployWatcher(ctx, osWriters, source, appDetails)
}

func (s Providers) FindStatuser(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (Statuser, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil || factory.NewStatuser == nil {
		return nil, nil
	}
	return factory.NewStatuser(ctx, osWriters, source, appDetails)
}

func (s Providers) FindLogStreamer(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails Details) (LogStreamer, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil || factory.NewLogStreamer == nil {
		return nil, nil
	}
	return factory.NewLogStreamer(ctx, osWriters, source, appDetails)
}

func (s Providers) FindFactory(curModule types.Module) *Provider {
	return contract.FindInRegistrarByModule(s, &curModule)
}
