package block

import (
	"github.com/nullstone-io/deployment-sdk/logging"
	"gopkg.in/nullstone-io/go-api-client.v0"
)

type Provider struct {
	NewMetricsGetter NewMetricsGetterFunc
}

type NewMetricsGetterFunc func(osWriters logging.OsWriters, nsConfig api.Config, blockDetails Details) (MetricsGetter, error)
