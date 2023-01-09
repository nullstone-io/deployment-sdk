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
func UpdateStandard(cur map[string]string, meta app.DeployMetadata) {
	std := GetStandard(meta)
	for k, v := range std {
		// We don't want to introduce new env vars, only modify existing
		if _, exists := cur[k]; exists {
			cur[k] = v
		}
	}
}
