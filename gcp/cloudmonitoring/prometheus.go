package cloudmonitoring

import (
	"context"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/prometheus"
	"golang.org/x/oauth2"
	"net/url"
)

func NewPrometheusClient(ctx context.Context, projectId string, tokenSource oauth2.TokenSource) *prometheus.QueryClient {
	baseUrl := &url.URL{
		Scheme: "https",
		Host:   "monitoring.googleapis.com",
		Path:   fmt.Sprintf("/v1/projects/%s/location/global/prometheus/", projectId),
	}
	httpClient := oauth2.NewClient(ctx, tokenSource)
	return &prometheus.QueryClient{
		BaseUrl:    baseUrl,
		HttpClient: httpClient,
	}
}
