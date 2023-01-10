package cdn

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/logging"
	"strings"
)

// UpdateCdnVersion updates the cloudfront distribution with the appropriate app version
// This returns a false result if no changes were made to the distribution
func UpdateCdnVersion(ctx context.Context, infra Outputs, version string) (bool, error) {
	cdns, err := GetCdns(ctx, infra)
	if err != nil {
		return false, err
	}

	hasChanges := false
	newOriginPath := coerceValidOriginPath(infra.ArtifactsKey(version))
	cfClient := nsaws.NewCloudfrontClient(infra.Deployer, infra.Region)
	for _, cdnRes := range cdns {
		changed, dc := calcDistributionConfig(ctx, cdnRes.Distribution, newOriginPath)
		if !changed || dc == nil {
			// We don't update the distribution if there were no changes or we don't support making changes
			continue
		}
		hasChanges = true
		_, err := cfClient.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{
			DistributionConfig: dc,
			Id:                 cdnRes.Distribution.Id,
			IfMatch:            cdnRes.ETag,
		})
		if err != nil {
			return false, fmt.Errorf("error updating distribution %q: %w", *cdnRes.Distribution.Id, err)
		}
	}

	return hasChanges, err
}

func coerceValidOriginPath(artifactsDir string) string {
	// Ensure there is a preceding `/` and no trailing `/`
	// Coerce value of `/` to empty string (`/` is invalid)
	dir := fmt.Sprintf("/%s", strings.TrimPrefix(artifactsDir, "/"))
	return strings.TrimSuffix(dir, "/")
}

// calcDistributionConfig makes changes to the distribution config for a deployment
// If the distribution does not have a default origin, we return a nil config which signifies that we don't support updates
// This also returns a bool that indicates whether any changes were made to the distribution config
func calcDistributionConfig(ctx context.Context, cdn *cftypes.Distribution, newOriginPath string) (bool, *cftypes.DistributionConfig) {
	stdout := logging.OsWritersFromContext(ctx).Stdout()

	index, defaultOrigin := findDefaultOrigin(cdn)
	if index < 0 {
		// This only knows how to update the version on the default origin for a CDN
		// If there is no default origin, skip it
		return false, nil
	}
	oldOriginPath := *defaultOrigin.OriginPath
	changed := oldOriginPath != newOriginPath
	dc := cdn.DistributionConfig
	dc.Origins.Items[index].OriginPath = aws.String(newOriginPath)
	fmt.Fprintf(stdout, "Updating CDN origin path to %q\n", newOriginPath)
	return changed, dc
}

func findDefaultOrigin(cdn *cftypes.Distribution) (int, cftypes.Origin) {
	defaultOriginId := getDefaultOriginId(cdn)
	if defaultOriginId == "" {
		return -1, cftypes.Origin{}
	}
	for i, item := range cdn.DistributionConfig.Origins.Items {
		if *item.Id == defaultOriginId {
			return i, item
		}
	}
	return -1, cftypes.Origin{}
}

func getDefaultOriginId(cdn *cftypes.Distribution) string {
	if cdn == nil || cdn.DistributionConfig == nil || cdn.DistributionConfig.DefaultCacheBehavior == nil {
		return ""
	}
	dcb := cdn.DistributionConfig.DefaultCacheBehavior
	if dcb.TargetOriginId == nil {
		return ""
	}
	return *dcb.TargetOriginId
}
