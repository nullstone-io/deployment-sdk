package env_vars

import "github.com/nullstone-io/deployment-sdk/app"

func GetStandard(meta app.DeployMetadata) map[string]string {
	return map[string]string{
		"NULLSTONE_VERSION":    meta.Version,
		"NULLSTONE_COMMIT_SHA": meta.CommitSha,
	}
}

func UpdateStandard(existing map[string]string, meta app.DeployMetadata) map[string]string {
	updated := existing
	for k, v := range GetStandard(meta) {
		updated[k] = v
	}
	return updated
}
