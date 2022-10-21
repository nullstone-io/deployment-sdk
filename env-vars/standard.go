package env_vars

import "github.com/nullstone-io/deployment-sdk/app"

func GetStandard(meta app.DeployMetadata) map[string]string {
	return map[string]string{
		"NULLSTONE_VERSION":    meta.Version,
		"NULLSTONE_COMMIT_SHA": meta.CommitSha,
	}
}

// UpdateStandard returns an updated version of the env vars using new application deploy metadata
// This does not add new env vars, only replaces existing ones
// Otherwise, this could cause thrashing between code deploys and terraform plans
// Essentially, we rely on the Terraform plan to be the source of truth for what env vars are included/excluded
func UpdateStandard(existing map[string]string, meta app.DeployMetadata) map[string]string {
	std := GetStandard(meta)
	updated := existing
	for k := range existing {
		if val, ok := std[k]; ok {
			std[k] = val
		}
	}
	return updated
}
