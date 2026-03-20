package creds

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"gopkg.in/nullstone-io/go-api-client.v0/types"
)

type ProviderFactory func(purpose string, outputNames ...string) aws.CredentialsProvider

func NewProviderFactory(source outputs.RetrieverSource, stackId, blockId, envId int64) ProviderFactory {
	return func(purpose string, outputNames ...string) aws.CredentialsProvider {
		return NullstoneCredsProvider{
			RetrieverSource: source,
			StackId:         stackId,
			BlockId:         blockId,
			EnvId:           envId,
			Purpose:         purpose,
			OutputNames:     outputNames,
		}
	}
}

var (
	_ aws.CredentialsProvider = NullstoneCredsProvider{}
)

type NullstoneCredsProvider struct {
	RetrieverSource outputs.RetrieverSource
	StackId         int64
	BlockId         int64
	EnvId           int64
	Purpose         string
	OutputNames     []string
}

func (p NullstoneCredsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	input := api.AcquireAutomationCredentialsInput{
		ProviderType: types.ProviderAws,
		Purpose:      p.Purpose,
		OutputNames:  p.OutputNames,
	}
	creds, err := p.RetrieverSource.GetAutomationCredentials(ctx, p.StackId, p.BlockId, p.EnvId, input)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("error retrieving temporary credentials from Nullstone: %w", err)
	}
	if creds == nil {
		return aws.Credentials{}, nil
	}
	return aws.Credentials{
		AccessKeyID:     creds.Aws.AccessKeyID,
		SecretAccessKey: creds.Aws.SecretAccessKey,
		SessionToken:    creds.Aws.SessionToken,
		Source:          creds.Aws.Source,
		CanExpire:       creds.Aws.CanExpire,
		Expires:         creds.Aws.Expires,
	}, nil
}
