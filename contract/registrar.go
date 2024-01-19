package contract

import (
	"github.com/nullstone-io/deployment-sdk/maps"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
	"sort"
)

// Registrar enables registration of various providers and utilities that are built to support modules that follow a specific contract
// This interface enables a simple registration interface to query information or execute commands based on a workspace module
type Registrar[T any] map[types.ModuleContractName]T

func FindInRegistrarByModule[T any](m map[types.ModuleContractName]T, module *types.Module) *T {
	if module == nil || len(module.ProviderTypes) <= 0 {
		return nil
	}

	contract := types.ModuleContractName{
		Category:    string(module.Category),
		Subcategory: string(module.Subcategory),
		// TODO: Enforce module provider can only contain one and only one provider type
		Provider:    module.ProviderTypes[0],
		Platform:    module.Platform,
		Subplatform: module.Subplatform,
	}

	r := Registrar[T](m)
	for _, mcn := range r.SortedKeys() {
		if mcn.Match(contract) {
			v := r[mcn]
			return &v
		}
	}
	return nil
}

// SortedKeys returns a list of provider keys sorted from more-specific to least-specific
// This guarantees that a more-specific module contract is found first when trying to match contracts
// Example: a Beanstalk app should match `app:server/aws/ec2:beanstalk` before `app:server/aws/ec2:*`
func (r Registrar[T]) SortedKeys() []types.ModuleContractName {
	keys := maps.Keys(r)
	sort.SliceStable(keys, func(i, j int) bool {
		return types.CompareModuleContractName(keys[i], keys[j])
	})
	return keys
}
