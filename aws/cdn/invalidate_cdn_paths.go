package cdn

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/nullstone-io/deployment-sdk/aws"
	"time"
)

func InvalidateCdnPaths(ctx context.Context, infra Outputs, urlPaths []string) ([]string, error) {
	cfClient := nsaws.NewCloudfrontClient(infra.Deployer, infra.Region)
	invalidationIds := make([]string, 0)
	for _, cdnId := range infra.CdnIds {
		out, err := cfClient.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
			DistributionId: aws.String(cdnId),
			InvalidationBatch: &cftypes.InvalidationBatch{
				CallerReference: aws.String(time.Now().String()),
				Paths: &cftypes.Paths{
					Quantity: aws.Int32(int32(len(urlPaths))),
					Items:    urlPaths,
				},
			},
		})
		if err != nil {
			return invalidationIds, fmt.Errorf("error invalidating cdn %s: %w", cdnId, err)
		} else if out != nil && out.Invalidation != nil {
			invalidationIds = append(invalidationIds, *out.Invalidation.Id)
		}
	}
	return invalidationIds, nil
}
