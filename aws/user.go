package nsaws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

var (
	_ aws.CredentialsProvider = User{}
)

// User contains credentials for a user that has access to perform a particular action in AWS
// This structure must match the fields defined in outputs of the module
type User struct {
	Name            string `json:"name"`
	AccessKeyId     string `json:"access_key"`
	SecretAccessKey string `json:"secret_key"`

	RemoteProvider aws.CredentialsProvider `json:"-"`
}

func (u User) Retrieve(ctx context.Context) (aws.Credentials, error) {
	// First, try static credentials
	provider := credentials.NewStaticCredentialsProvider(u.AccessKeyId, u.SecretAccessKey, "")
	creds, err := provider.Retrieve(ctx)
	if err == nil {
		return creds, nil
	}

	// If static credentials are empty, it will fall back to the remote provider
	if u.RemoteProvider != nil {
		return u.RemoteProvider.Retrieve(ctx)
	}

	return aws.Credentials{}, fmt.Errorf("missing AWS credentials")
}
