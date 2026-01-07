package cloudfunctions

import (
	"context"

	cloudfunctions "cloud.google.com/go/functions/apiv1"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"google.golang.org/api/option"
)

var GcpScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
}

func NewCloudFunctionsClient(ctx context.Context, account gcp.ServiceAccount) (*cloudfunctions.CloudFunctionsClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return cloudfunctions.NewCloudFunctionsClient(ctx, option.WithTokenSource(tokenSource))
}
