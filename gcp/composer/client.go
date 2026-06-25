package composer

import (
	"context"

	service "cloud.google.com/go/orchestration/airflow/service/apiv1"
	"github.com/nullstone-io/deployment-sdk/gcp"
	"google.golang.org/api/option"
)

var GcpScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
}

// NewEnvironmentsClient creates a Cloud Composer EnvironmentsClient authenticated with the given service account.
func NewEnvironmentsClient(ctx context.Context, account gcp.ServiceAccount) (*service.EnvironmentsClient, error) {
	tokenSource, err := account.TokenSource(ctx, GcpScopes...)
	if err != nil {
		return nil, err
	}
	return service.NewEnvironmentsClient(ctx, option.WithTokenSource(tokenSource))
}
