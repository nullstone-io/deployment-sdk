package cdn

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func UpdateCdnVersion(ctx context.Context, infra Outputs, version string) error {
	cdns, err := GetCdns(ctx, infra)
	if err != nil {
		return err
	}

	cfClient := nsaws.NewCloudfrontClient(infra.Deployer, infra.Region)
	for _, cdnRes := range cdns {
		_, err := cfClient.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{
			DistributionConfig: replaceOriginPath(cdnRes, version),
			Id:                 cdnRes.Distribution.Id,
			IfMatch:            cdnRes.ETag,
		})
		if err != nil {
			return fmt.Errorf("error updating distribution %q: %w", *cdnRes.Distribution.Id, err)
		}
	}

	return err
}

func replaceOriginPath(cdn *cloudfront.GetDistributionOutput, newOriginPath string) *cftypes.DistributionConfig {
	primaryOriginId := getDefaultOriginId(cdn.Distribution)
	dc := cdn.Distribution.DistributionConfig
	if primaryOriginId == "" {
		return dc
	}

	for i, item := range dc.Origins.Items {
		if *item.Id == primaryOriginId {
			dc.Origins.Items[i].OriginPath = aws.String(fmt.Sprintf("/%s", newOriginPath))
		}
	}
	return dc
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
