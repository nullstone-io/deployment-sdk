package composer

import (
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/gcp/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	ProjectId       string `ns:"project_id"`
	Region          string `ns:"region"`
	EnvironmentName string `ns:"environment_name"`
	// DagGcsPrefix is the GCS location where DAGs are uploaded for this environment.
	// It has the form gs://<bucket>/dags.
	DagGcsPrefix string `ns:"dag_gcs_prefix"`
	// DagGcsBucket is the name of the Composer-managed GCS bucket that holds this environment's DAGs.
	DagGcsBucket string `ns:"dag_gcs_bucket,optional"`

	// Deployer impersonates the Composer environment to update its software configuration (env variables).
	Deployer gcp.ServiceAccount `ns:"deployer"`
	// Pusher syncs DAG files to the Composer-managed GCS bucket.
	Pusher gcp.ServiceAccount `ns:"pusher"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.Deployer.RemoteTokenSourcer = creds.NewTokenSourcer(source, ws.StackId, ws.BlockId, ws.EnvId, types.AutomationPurposeDeploy, "deployer")
	o.Pusher.RemoteTokenSourcer = creds.NewTokenSourcer(source, ws.StackId, ws.BlockId, ws.EnvId, types.AutomationPurposePush, "pusher")
}
