package cloudcdn

import (
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"fmt"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/logging"
	"google.golang.org/api/option"
	"strings"
	"time"
)

// UpdateCdnVersion updates the cloudfront distribution with the appropriate app version
// This returns a false result if no changes were made to the distribution
func UpdateCdnVersion(ctx context.Context, infra Outputs, version string) (bool, error) {
	tokenSource, err := infra.Deployer.TokenSource(ctx, CdnScopes...)
	if err != nil {
		return false, fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := compute.NewUrlMapsRESTClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return false, fmt.Errorf("error creating google compute client: %w", err)
	}
	defer client.Close()

	urlMaps, err := GetUrlMaps(ctx, infra, client)
	if err != nil {
		return false, err
	}
	hasChanges := false
	newPathPrefix := coerceValidPathPrefix(infra.ArtifactsKey(version))
	for _, urlMap := range urlMaps {
		changed := updateUrlMapPathPrefix(ctx, urlMap, newPathPrefix)
		if !changed {
			// We don't update the distribution if there were no changes or we don't support making changes
			continue
		}
		hasChanges = true
		requestId := time.Now().String()
		req := &computepb.UpdateUrlMapRequest{
			Project:        infra.ProjectId,
			RequestId:      &requestId,
			UrlMap:         *urlMap.Name,
			UrlMapResource: urlMap,
		}
		_, err := client.Update(ctx, req)
		if err != nil {
			return false, fmt.Errorf("error updating url map %q: %w", *urlMap.Name, err)
		}
	}

	return hasChanges, err
}

func coerceValidPathPrefix(artifactsDir string) string {
	if artifactsDir == "" {
		return ""
	}
	// Ensure there is a single preceding `/`
	if !strings.HasPrefix(artifactsDir, "/") {
		artifactsDir = "/" + artifactsDir
	}
	// Drop trailing '/'
	artifactsDir = strings.TrimSuffix(artifactsDir, "/")
	return artifactsDir
}

func updateUrlMapPathPrefix(ctx context.Context, urlMap *computepb.UrlMap, newPathPrefix string) bool {
	stdout := logging.OsWritersFromContext(ctx).Stdout()

	for _, pathMatcher := range urlMap.PathMatchers {
		if pathMatcher.DefaultRouteAction != nil && pathMatcher.DefaultRouteAction.UrlRewrite != nil {
			ur := pathMatcher.DefaultRouteAction.UrlRewrite
			existing := ur.PathPrefixRewrite
			if existing != nil && *existing == newPathPrefix {
				colorstring.Fprintf(stdout, "Path rule in url map (%s) is already configured with the correct app version\n", *urlMap.Name)
				return false
			}
			ur.PathPrefixRewrite = &newPathPrefix
			return true
		}
	}
	colorstring.Fprintf(stdout, "Could not find valid path rule in url map (%s) to update app version\n", *urlMap.Name)
	return false
}
