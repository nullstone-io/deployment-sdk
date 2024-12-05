package cloudcdn

import (
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"fmt"
)

func GetUrlMaps(ctx context.Context, infra Outputs, client *compute.UrlMapsClient) ([]*computepb.UrlMap, error) {
	result := make([]*computepb.UrlMap, 0)
	for _, urlMapName := range infra.CdnUrlMapNames {
		req := &computepb.GetUrlMapRequest{
			Project: infra.ProjectId,
			UrlMap:  urlMapName,
		}
		urlMap, err := client.Get(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("error getting url map %s: %w", urlMapName, err)
		} else if urlMap != nil {
			result = append(result, urlMap)
		}
	}
	return result, nil
}
