package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/nullstone-io/deployment-sdk/azure/creds"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

var _ azcore.TokenCredential = &Principal{}

// Principal holds Azure credentials for a service principal using Workload Identity Federation.
// TenantId and ClientId identify the App Registration; token retrieval is delegated to Nullstone.
type Principal struct {
	TenantId string `json:"tenant_id"`
	ClientId string `json:"client_id"`

	RemoteCredential creds.CredentialFactory `json:"-"`
}

func (p *Principal) InitializeCreds(source outputs.RetrieverSource, ws *types.Workspace, purpose string, outputNames ...string) {
	p.RemoteCredential = creds.NewCredentialFactory(source, ws.StackId, ws.BlockId, ws.EnvId, purpose, outputNames...)
}

func (p *Principal) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if p.RemoteCredential == nil {
		return azcore.AccessToken{}, fmt.Errorf("missing Azure credentials: no remote credential factory configured")
	}
	cred := p.RemoteCredential(options.Scopes)
	return cred.GetToken(ctx, options)
}
