package cloudrun

import (
	"context"

	run "cloud.google.com/go/run/apiv2"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"google.golang.org/api/option"
)

var GcpScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
}

func NewJobsClient(ctx context.Context, account gcp.ServiceAccount) (*run.JobsClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return run.NewJobsClient(ctx, option.WithTokenSource(tokenSource))
}

func NewServicesClient(ctx context.Context, account gcp.ServiceAccount) (*run.ServicesClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return run.NewServicesClient(ctx, option.WithTokenSource(tokenSource))
}
