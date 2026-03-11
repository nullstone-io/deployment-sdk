package nsaws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// IamIdentity holds configuration for an IAM User or IAM Role
// If AccessKeyId is not empty -> IAM User
// If RoleArn is not empty -> IAM Role
type IamIdentity struct {
	SessionDuration int32 `json:"session_duration"`

	// Used for GetSessionToken
	Name            string `json:"name"`
	AccessKeyId     string `json:"access_key"`
	SecretAccessKey string `json:"secret_key"`

	// Used for AssumeRole
	RoleArn    string `json:"role_arn"`
	ExternalId string `json:"external_id"`

	RemoteProvider aws.CredentialsProvider `json:"-"`
}

func (i IamIdentity) Validate() error {
	if i.AccessKeyId == "" && i.RoleArn == "" {
		return fmt.Errorf("output does not contain 'access_key_id' (IAM User) or 'role_arn' (IAM Role)")
	}
	return nil
}

func (i IamIdentity) Retrieve(ctx context.Context) (aws.Credentials, error) {
	if i.AccessKeyId != "" {
		// First, try static credentials
		provider := credentials.NewStaticCredentialsProvider(i.AccessKeyId, i.SecretAccessKey, "")
		creds, err := provider.Retrieve(ctx)
		if err == nil {
			return creds, nil
		}
	}

	// If static credentials are empty, it will fall back to the remote provider
	// This will handle AssumeRole with RoleArn via Nullstone API
	if i.RemoteProvider != nil {
		return i.RemoteProvider.Retrieve(ctx)
	}

	return aws.Credentials{}, fmt.Errorf("missing AWS credentials")
}
