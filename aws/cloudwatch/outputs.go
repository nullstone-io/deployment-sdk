package cloudwatch

import (
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	Region       string            `ns:"region"`
	LogReader    nsaws.IamIdentity `ns:"log_reader"`
	LogGroupName string            `ns:"log_group_name"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	credsFactory := creds.NewProviderFactory(source, ws.StackId, ws.BlockId, ws.EnvId)
	o.LogReader.RemoteProvider = credsFactory(types.AutomationPurposeViewLogs, "log_reader")
}
