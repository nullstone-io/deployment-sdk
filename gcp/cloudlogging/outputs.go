package cloudlogging

import (
	"github.com/nullstone-io/deployment-sdk/gcp"
	"github.com/nullstone-io/deployment-sdk/gcp/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	ProjectId string             `ns:"project_id"`
	LogFilter string             `ns:"log_filter"`
	LogReader gcp.ServiceAccount `ns:"log_reader"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.LogReader.RemoteTokenSourcer = creds.NewTokenSourcer(source, ws.StackId, ws.Uid, "log_reader")
}
