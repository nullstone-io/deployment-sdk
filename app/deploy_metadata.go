package app

type DeployMetadata struct {
	Repo        string
	Version     string
	CommitSha   string
	Type        string
	PackageMode string
}
