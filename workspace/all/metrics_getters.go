package all

import (
	cwMetrics "github.com/nullstone-io/deployment-sdk/aws/cloudwatch/metrics"
	"github.com/nullstone-io/deployment-sdk/gcp/cloudmonitoring"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var (
	Aws = types.ModuleContractName{
		Category: "*",
		Provider: "aws",
		Platform: "*",
	}
	Gcp = types.ModuleContractName{
		Category: "*",
		Provider: "gcp",
		Platform: "*",
	}
	// MetricsGetters is a factory for creating a new MetricsGetter from a workspace
	// If the factory method returns an error, it is wrapped with MetricsNotSupportedError
	MetricsGetters = workspace.MetricsGetters{
		Aws: cwMetrics.NewGetter,
		Gcp: cloudmonitoring.NewGetter,
	}
)
