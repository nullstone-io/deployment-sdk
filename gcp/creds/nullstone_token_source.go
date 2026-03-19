package creds

import (
	"context"
	"fmt"

	"github.com/nullstone-io/deployment-sdk/outputs"
	"golang.org/x/oauth2"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type TokenSourcer func(scopes []string) oauth2.TokenSource

func NewTokenSourcer(source outputs.RetrieverSource, stackId, blockId, envId int64, purpose string, outputNames ...string) TokenSourcer {
	return func(scopes []string) oauth2.TokenSource {
		return NullstoneTokenSource{
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

var (
	_ oauth2.TokenSource = NullstoneTokenSource{}
)

type NullstoneTokenSource struct {
	RetrieverSource outputs.RetrieverSource
	StackId         int64
	BlockId         int64
	EnvId           int64
	Purpose         string
	OutputNames     []string
	Scopes          []string
}

func (p NullstoneTokenSource) Token() (*oauth2.Token, error) {
	input := api.AcquireAutomationCredentialsInput{
		ProviderType: types.ProviderGcp,
		Purpose:      p.Purpose,
		OutputNames:  p.OutputNames,
		OauthScopes:  p.Scopes,
	}
	creds, err := p.RetrieverSource.GetAutomationCredentials(context.TODO(), p.StackId, p.BlockId, p.EnvId, input)
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
