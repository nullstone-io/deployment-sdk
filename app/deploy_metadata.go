package app

type DeployMetadata struct {
	Repo        string
	Version     string
	CommitSha   string
	Type        string
	PackageMode string

	// EnvVars are additional environment variables supplied at deploy time (e.g. `nullstone deploy --env-var`)
	// These are applied to the app's infra resources (ECS task definition, k8s Deployment, etc.) for this deploy only.
	// They are not persisted, so a subsequent IaC run is the source of truth and will overwrite them.
	EnvVars map[string]string
}
