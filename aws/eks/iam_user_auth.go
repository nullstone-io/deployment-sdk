package eks

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	nsaws "github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/k8s"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

var _ k8s.AuthInfoer = IamUserAuth{}

type IamUserAuth struct {
	nsaws.User
	Region    string
	ClusterId string
}

func (i IamUserAuth) AuthInfo(ctx context.Context) (clientcmdapi.AuthInfo, error) {
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return clientcmdapi.AuthInfo{}, err
	}
	ses, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(i.User.AccessKeyId, i.User.SecretAccessKey, ""),
		Region:      aws.String(i.Region),
	})
	opts := &token.GetTokenOptions{
		ClusterID: i.ClusterId,
		Session:   ses,
	}
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		return clientcmdapi.AuthInfo{}, err
	}

	return clientcmdapi.AuthInfo{
		Token: tok.Token,
	}, nil
}
