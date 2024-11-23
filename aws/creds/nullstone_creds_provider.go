package creds

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/uuid"
	"github.com/nullstone-io/deployment-sdk/outputs"
)

type ProviderFactory func(outputNames ...string) aws.CredentialsProvider

func NewProviderFactory(source outputs.RetrieverSource, stackId int64, workspaceUid uuid.UUID) ProviderFactory {
	return func(outputNames ...string) aws.CredentialsProvider {
		return NullstoneCredsProvider{
			RetrieverSource: source,
			StackId:         stackId,
			WorkspaceUid:    workspaceUid,
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
	WorkspaceUid    uuid.UUID
	OutputNames     []string
}

func (p NullstoneCredsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := p.RetrieverSource.GetTemporaryCredentials(ctx, p.StackId, p.WorkspaceUid, p.OutputNames)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("error retrieving temporary credentials from Nullstone: %w", err)
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
