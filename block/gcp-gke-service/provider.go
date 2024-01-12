package gcp_gke_service

import (
	"github.com/nullstone-io/deployment-sdk/block"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var ModuleContractName = types.ModuleContractName{
	Category:    string(types.CategoryApp),
	Subcategory: string(types.SubcategoryAppContainer),
	Provider:    "gcp",
	Platform:    "k8s",
	Subplatform: "gke",
}

var Provider = block.Provider{
	NewMetricsGetter: nil,
}
