package creds

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/google/uuid"
	"github.com/nullstone-io/deployment-sdk/outputs"
	api "gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

// CredentialFactory creates an azcore.TokenCredential bound to specific OAuth2 scopes.
// This parallels gcp/creds.TokenSourcer.
type CredentialFactory func(scopes []string) azcore.TokenCredential

func NewCredentialFactory(source outputs.RetrieverSource, stackId int64, workspaceUid uuid.UUID, outputNames ...string) CredentialFactory {
	return func(scopes []string) azcore.TokenCredential {
		return NullstoneCredsProvider{
			RetrieverSource: source,
			StackId:         stackId,
			WorkspaceUid:    workspaceUid,
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
	WorkspaceUid    uuid.UUID
	Scopes          []string
	OutputNames     []string
}

func (p NullstoneCredsProvider) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	scopes := options.Scopes
	if len(scopes) == 0 {
		scopes = p.Scopes
	}
	input := api.GenerateCredentialsInput{
		Provider:         types.ProviderAzure,
		OutputNames:      p.OutputNames,
		AzureOauthScopes: scopes,
	}
	creds, err := p.RetrieverSource.GetTemporaryCredentials(ctx, p.StackId, p.WorkspaceUid, input)
	if err != nil {
		return azcore.AccessToken{}, fmt.Errorf("error retrieving temporary credentials from Nullstone: %w", err)
	}
	if creds == nil || creds.Azure == nil {
		return azcore.AccessToken{}, fmt.Errorf("no Azure credentials returned from Nullstone")
	}
	return azcore.AccessToken{
		Token:     creds.Azure.AccessToken,
		ExpiresOn: creds.Azure.ExpiresOn,
	}, nil
}
