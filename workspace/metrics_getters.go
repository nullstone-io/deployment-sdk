package workspace

import (
	"github.com/nullstone-io/deployment-sdk/logging"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type NewMetricsGetterFunc func(osWriters logging.OsWriters, nsConfig api.Config, blockDetails Details) (MetricsGetter, error)

type MetricsGetters map[types.ModuleContractName]NewMetricsGetterFunc

func (s MetricsGetters) FindMetricsGetter(osWriters logging.OsWriters, nsConfig api.Config, blockDetails Details) (MetricsGetter, error) {
	curModule := blockDetails.Module
	if len(curModule.ProviderTypes) <= 0 {
		return nil, nil
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
			if v == nil {
				return nil, nil
			}
			return v(osWriters, nsConfig, blockDetails)
		}
	}

	return nil, nil
}
