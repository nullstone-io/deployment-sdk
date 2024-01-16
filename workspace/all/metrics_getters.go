package all

import (
	cwMetrics "github.com/nullstone-io/deployment-sdk/aws/cloudwatch/metrics"
	"github.com/nullstone-io/deployment-sdk/workspace"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var (
	Aws = types.ModuleContractName{
		Category: "*",
		Provider: "aws",
		Platform: "*",
	}
	GcpGke = types.ModuleContractName{
		Category:    string(types.CategoryApp),
		Subcategory: string(types.SubcategoryAppContainer),
		Provider:    "gcp",
		Platform:    "k8s",
		Subplatform: "gke",
	}
	MetricsGetters = workspace.MetricsGetters{
		Aws:    cwMetrics.NewGetter,
		GcpGke: nil,
	}
)
