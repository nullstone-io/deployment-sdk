package block

import (
	"github.com/nullstone-io/deployment-sdk/logging"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Providers map[types.ModuleContractName]Provider

func (s Providers) FindMetricsGetter(osWriters logging.OsWriters, nsConfig api.Config, appDetails Details) (MetricsGetter, error) {
	factory := s.FindFactory(*appDetails.Module)
	if factory == nil || factory.NewMetricsGetter == nil {
		return nil, nil
	}
	return factory.NewMetricsGetter(osWriters, nsConfig, appDetails)
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
