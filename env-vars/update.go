package env_vars

import "github.com/nullstone-io/deployment-sdk/app"

func Update(existing map[string]string, meta app.DeployMetadata) map[string]string {
	updated := existing
	updated["NULLSTONE_VERSION"] = meta.Version
	updated["NULLSTONE_COMMIT_SHA"] = meta.CommitSha
	return updated
}
