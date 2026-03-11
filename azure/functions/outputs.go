package functions

import (
	"github.com/nullstone-io/deployment-sdk/azure"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type Outputs struct {
	SubscriptionId  string          `ns:"subscription_id"`
	ResourceGroup   string          `ns:"resource_group"`
	FunctionAppName string          `ns:"function_app_name"`
	Deployer        azure.Principal `ns:"deployer"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.Deployer.InitializeCreds(source, ws, "deployer")
}
