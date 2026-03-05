package cloudcdn

import (
	"context"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/google/uuid"
	"github.com/mitchellh/colorstring"
	"github.com/nullstone-io/deployment-sdk/logging"
	"google.golang.org/api/option"
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
	for _, urlMap := range urlMaps {
		changed := updateUrlMapPathPrefix(ctx, urlMap, version)
		if !changed {
			// We don't update the distribution if there were no changes or we don't support making changes
			continue
		}
		hasChanges = true
		requestId := uuid.New().String()
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

func updateUrlMapPathPrefix(ctx context.Context, urlMap *computepb.UrlMap, newVersion string) bool {
	stdout := logging.OsWritersFromContext(ctx).Stdout()

	for _, pathMatcher := range urlMap.PathMatchers {
		oldVersion := updateNullstoneVersionHeader(pathMatcher, newVersion)
		if oldVersion == "" {
			// We only update if we found X-Nullstone-Version header in this path matcher
			continue
		}

		colorstring.Fprintf(stdout, "Updating path matcher %q\n", pathMatcher.GetName())
		for i, routeRule := range pathMatcher.RouteRules {
			if routeRule.RouteAction != nil && routeRule.RouteAction.UrlRewrite != nil {
				ur := routeRule.RouteAction.UrlRewrite
				ur.PathPrefixRewrite = replaceVersion(routeRule.RouteAction.UrlRewrite.PathPrefixRewrite, oldVersion, newVersion)
				if ur.PathPrefixRewrite != nil {
					colorstring.Fprintf(stdout, "Updated route_rules[%d].route_action.url_rewrite.path_prefix_rewrite with %q\n", i, *ur.PathPrefixRewrite)
				}
			}
		}
		if pathMatcher.DefaultRouteAction != nil && pathMatcher.DefaultRouteAction.UrlRewrite != nil {
			ur := pathMatcher.DefaultRouteAction.UrlRewrite
			ur.PathPrefixRewrite = replaceVersion(pathMatcher.DefaultRouteAction.UrlRewrite.PathPrefixRewrite, oldVersion, newVersion)
			if ur.PathPrefixRewrite != nil {
				colorstring.Fprintf(stdout, "Updated default_route_action.url_rewrite.path_prefix_rewrite with %q\n", *ur.PathPrefixRewrite)
			}
			return true
		}
		if pathMatcher.DefaultCustomErrorResponsePolicy != nil {
			for i, errorPolicy := range pathMatcher.DefaultCustomErrorResponsePolicy.ErrorResponseRules {
				errorPolicy.Path = replaceVersion(errorPolicy.Path, oldVersion, newVersion)
				if errorPolicy.Path != nil {
					colorstring.Fprintf(stdout, "Updated default_custom_error_response_policy.error_response_rule[%d].path with %q\n", i, *errorPolicy.Path)
				}
			}
		}
	}
	return false
}

const (
	appVersionHeaderName = "X-Nullstone-Version"
)

func updateNullstoneVersionHeader(matcher *computepb.PathMatcher, newVersion string) string {
	if matcher.HeaderAction == nil {
		return ""
	}
	for _, cur := range matcher.HeaderAction.RequestHeadersToAdd {
		if cur.GetHeaderName() == appVersionHeaderName {
			result := cur.GetHeaderValue()
			cur.HeaderValue = &newVersion
			return result
		}
	}
	return ""
}

func replaceVersion(pathValue *string, oldVersion, newVersion string) *string {
	if pathValue == nil {
		return nil
	}
	if *pathValue == "" || oldVersion == "" || newVersion == "" {
		return pathValue
	}
	updated := strings.ReplaceAll(*pathValue, oldVersion, newVersion)
	return &updated
}
