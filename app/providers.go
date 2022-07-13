package app

import (
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"log"
)

type ProviderFactory func(logger *log.Logger, nsConfig api.Config, appDetails Details) Provider

type Providers map[types.ModuleContractName]Provider

func (s Providers) FindPusher(logger *log.Logger, nsConfig api.Config, appDetails Details) (Pusher, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil {
		return nil, nil
	}
	return factory.NewPusher()
}

func (s Providers) FindDeployer(logger *log.Logger, nsConfig api.Config, appDetails Details) (Deployer, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil {
		return nil, nil
	}
	return factory.NewDeployer()
}

func (s Providers) FindDeployStatusGetter(logger *log.Logger, nsConfig api.Config, appDetails Details) (DeployStatusGetter, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil {
		return nil, nil
	}
	return factory.NewDeployStatusGetter()
}

func (s Providers) FindFactory(curModule types.Module) *Provider {
	if len(curModule.ProviderTypes) <= 0 {
		return nil
	}

	// NOTE: We are matching app modules, so category is redundant
	//   However, this should guard against non-app modules trying to use these app providers
	curContract := types.ModuleContractName{
		Category:    string(curModule.Category),
		Subcategory: string(curModule.Subcategory),
		// TODO: Enforce module provider can only contain one and only one provider type
		Provider:    curModule.ProviderTypes[0],
		Platform:    curModule.Platform,
		Subplatform: curModule.Subplatform,
	}
	for k, v := range s {
		if k.Match(curContract) {
			return &v
		}
	}

	return nil
}
