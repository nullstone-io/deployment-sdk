package creds

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"golang.org/x/oauth2"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type TokenSourcer func(scopes []string) oauth2.TokenSource

func NewTokenSourcer(source outputs.RetrieverSource, stackId int64, workspaceUid uuid.UUID, outputNames ...string) TokenSourcer {
	return func(scopes []string) oauth2.TokenSource {
		return NullstoneTokenSource{
			RetrieverSource: source,
			StackId:         stackId,
			WorkspaceUid:    workspaceUid,
			Scopes:          scopes,
			OutputNames:     outputNames,
		}
	}
}

var (
	_ oauth2.TokenSource = NullstoneTokenSource{}
)

type NullstoneTokenSource struct {
	RetrieverSource outputs.RetrieverSource
	StackId         int64
	WorkspaceUid    uuid.UUID
	Scopes          []string
	OutputNames     []string
}

func (p NullstoneTokenSource) Token() (*oauth2.Token, error) {
	input := api.GenerateCredentialsInput{
		Provider:       types.ProviderGcp,
		OutputNames:    p.OutputNames,
		GcpOauthScopes: p.Scopes,
	}
	creds, err := p.RetrieverSource.GetTemporaryCredentials(context.TODO(), p.StackId, p.WorkspaceUid, input)
	if err != nil {
		return nil, fmt.Errorf("error retrieving temporary credentials from Nullstone: %w", err)
	}
	if creds == nil {
		return nil, nil
	}
	return &oauth2.Token{
		AccessToken:  creds.Gcp.AccessToken,
		TokenType:    creds.Gcp.TokenType,
		RefreshToken: creds.Gcp.RefreshToken,
		Expiry:       creds.Gcp.Expiry,
	}, nil
}
