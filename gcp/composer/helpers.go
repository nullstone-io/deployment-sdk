package composer

import (
	"fmt"
	"strings"
)

// environmentResourceName returns the fully-qualified Composer environment name used by the Composer API:
//
//	projects/{project}/locations/{region}/environments/{name}
func environmentResourceName(o Outputs) string {
	return fmt.Sprintf("projects/%s/locations/%s/environments/%s", o.ProjectId, o.Region, o.EnvironmentName)
}

// dagBucket returns the name of the GCS bucket that backs this environment's DAGs.
// It prefers the explicit dag_gcs_bucket output and falls back to parsing dag_gcs_prefix (gs://<bucket>/<prefix>).
func dagBucket(o Outputs) string {
	if o.DagGcsBucket != "" {
		return o.DagGcsBucket
	}
	bucket, _ := splitGcsUri(o.DagGcsPrefix)
	return bucket
}

// dagObjectPrefix returns the object key prefix within the DAG bucket where DAGs live (e.g. "dags").
func dagObjectPrefix(o Outputs) string {
	_, prefix := splitGcsUri(o.DagGcsPrefix)
	return prefix
}

// splitGcsUri splits a gs://<bucket>/<prefix> uri into its bucket and object prefix.
func splitGcsUri(uri string) (bucket string, prefix string) {
	trimmed := strings.TrimPrefix(uri, "gs://")
	bucket, prefix, _ = strings.Cut(trimmed, "/")
	prefix = strings.Trim(prefix, "/")
	return bucket, prefix
}
