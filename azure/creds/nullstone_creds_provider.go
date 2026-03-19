package creds

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

// CredentialFactory creates an azcore.TokenCredential bound to specific OAuth2 scopes.
// This parallels gcp/creds.TokenSourcer.
type CredentialFactory func(scopes []string) azcore.TokenCredential

func NewCredentialFactory(source outputs.RetrieverSource, stackId, blockId, envId int64, purpose string, outputNames ...string) CredentialFactory {
	return func(scopes []string) azcore.TokenCredential {
		return NullstoneCredsProvider{
			RetrieverSource: source,
			StackId:         stackId,
			BlockId:         blockId,
			EnvId:           envId,
			Purpose:         purpose,
			Scopes:          scopes,
			OutputNames:     outputNames,
		}
	}
}

var _ azcore.TokenCredential = NullstoneCredsProvider{}

// NullstoneCredsProvider implements azcore.TokenCredential by calling the Nullstone API
// to exchange a Nullstone OIDC assertion for a short-lived Azure access token.
type NullstoneCredsProvider struct {
	RetrieverSource outputs.RetrieverSource
	StackId         int64
	BlockId         int64
	EnvId           int64
	Purpose         string
	OutputNames     []string
	Scopes          []string
}

func (p NullstoneCredsProvider) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	scopes := options.Scopes
	if len(scopes) == 0 {
		scopes = p.Scopes
	}
	input := api.AcquireAutomationCredentialsInput{
		ProviderType: types.ProviderAzure,
		Purpose:      p.Purpose,
		OutputNames:  p.OutputNames,
		OauthScopes:  scopes,
	}
	creds, err := p.RetrieverSource.GetAutomationCredentials(ctx, p.StackId, p.BlockId, p.EnvId, input)
	if err != nil {
		return azcore.AccessToken{}, fmt.Errorf("error retrieving automation credentials from Nullstone: %w", err)
	}
	if creds == nil || creds.Azure == nil {
		return azcore.AccessToken{}, fmt.Errorf("no Azure credentials returned from Nullstone")
	}
	return azcore.AccessToken{
		Token:     creds.Azure.Token,
		ExpiresOn: creds.Azure.ExpiresOn,
		RefreshOn: creds.Azure.RefreshOn,
	}, nil
}
