package cloudcdn

import (
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

var (
	CdnScopes = []string{
		"https://www.googleapis.com/auth/compute",
	}
)

func InvalidateCdnPaths(ctx context.Context, infra Outputs, urlPaths []string) ([]string, error) {
	tokenSource, err := infra.Deployer.TokenSource(ctx, CdnScopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := compute.NewUrlMapsRESTClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("error creating google compute client: %w", err)
	}
	defer client.Close()

	invalidationNames := make([]string, 0)
	requestId := uuid.New().String()
	for _, urlPath := range urlPaths {
		for _, urlMapId := range infra.CdnUrlMapNames {
			req := &computepb.InvalidateCacheUrlMapRequest{
				Project:   infra.ProjectId,
				UrlMap:    urlMapId,
				RequestId: &requestId,
				CacheInvalidationRuleResource: &computepb.CacheInvalidationRule{
					Path: &urlPath,
				},
			}
			op, err := client.InvalidateCache(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("error invalidating url map %s: %w", urlMapId, err)
			}
			invalidationNames = append(invalidationNames, op.Name())
		}
	}
	return invalidationNames, nil
}
