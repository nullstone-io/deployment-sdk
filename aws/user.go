package nsaws

import (
	"context"
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
}

func (u User) Retrieve(ctx context.Context) (aws.Credentials, error) {
	provider := credentials.NewStaticCredentialsProvider(u.AccessKeyId, u.SecretAccessKey, "")
	return provider.Retrieve(ctx)
}
