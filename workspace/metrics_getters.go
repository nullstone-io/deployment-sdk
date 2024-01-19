package workspace

import (
	"github.com/nullstone-io/deployment-sdk/contract"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type NewMetricsGetterFunc func(osWriters logging.OsWriters, source outputs.RetrieverSource, blockDetails Details) (MetricsGetter, error)

type MetricsGetters map[types.ModuleContractName]NewMetricsGetterFunc

func (s MetricsGetters) FindMetricsGetter(osWriters logging.OsWriters, source outputs.RetrieverSource, blockDetails Details) (MetricsGetter, error) {
	fn := contract.FindInRegistrarByModule(s, blockDetails.Module)
	if fn == nil {
		return nil, nil
	}
	mg, err := (*fn)(osWriters, source, blockDetails)
	if err != nil {
		return nil, MetricsNotSupportedError{InnerErr: err}
	}
	return mg, nil
}
