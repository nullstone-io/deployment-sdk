package cloudrun

import (
	"context"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
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

func NewExecutionsClient(ctx context.Context, account gcp.ServiceAccount) (*run.ExecutionsClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return run.NewExecutionsClient(ctx, option.WithTokenSource(tokenSource))
}

func NewServicesClient(ctx context.Context, account gcp.ServiceAccount) (*run.ServicesClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return run.NewServicesClient(ctx, option.WithTokenSource(tokenSource))
}

func NewRevisionsClient(ctx context.Context, account gcp.ServiceAccount) (*run.RevisionsClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return run.NewRevisionsClient(ctx, option.WithTokenSource(tokenSource))
}

func NewTasksClient(ctx context.Context, account gcp.ServiceAccount) (*run.TasksClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return run.NewTasksClient(ctx, option.WithTokenSource(tokenSource))
}

func NewMetricClient(ctx context.Context, account gcp.ServiceAccount) (*monitoring.MetricClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return monitoring.NewMetricClient(ctx, option.WithTokenSource(tokenSource))
}
