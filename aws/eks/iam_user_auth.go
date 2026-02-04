package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sts"
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
	stsClient := sts.NewFromConfig(nsaws.NewConfig(i.User, i.Region))
	generator, err := token.NewGenerator(false, false)
	if err != nil {
		return clientcmdapi.AuthInfo{}, fmt.Errorf("failed to create token generator: %w", err)
	}

	tok, err := generator.GetWithSTS(i.ClusterId, stsClient)
	if err != nil {
		return clientcmdapi.AuthInfo{}, fmt.Errorf("failed to generate token: %w", err)
	}
	return clientcmdapi.AuthInfo{
		Token: tok.Token,
	}, nil
}
