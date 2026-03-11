package blob

import (
	"strings"

	"github.com/nullstone-io/deployment-sdk/azure"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

const (
	KeyTemplateAppVersion = "{{app-version}}"
)

type Outputs struct {
	SubscriptionId       string          `ns:"subscription_id"`
	ResourceGroup        string          `ns:"resource_group"`
	StorageAccount       string          `ns:"storage_account"`
	ContainerName        string          `ns:"container_name"`
	CdnProfileName       string          `ns:"cdn_profile_name,optional"`
	CdnEndpointName      string          `ns:"cdn_endpoint_name,optional"`
	ArtifactsKeyTemplate string          `ns:"artifacts_key_template"`
	Deployer             azure.Principal `ns:"deployer"`
}

func (o *Outputs) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace) {
	o.Deployer.InitializeCreds(source, ws, "deployer")
}

func (o *Outputs) ArtifactsKey(appVersion string) string {
	return strings.Replace(o.ArtifactsKeyTemplate, KeyTemplateAppVersion, appVersion, -1)
}

// BlobEndpoint returns the Azure Blob storage endpoint URL for this storage account.
func (o *Outputs) BlobEndpoint() string {
	return "https://" + o.StorageAccount + ".blob.core.windows.net"
}
