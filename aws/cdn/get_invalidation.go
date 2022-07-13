package cdn

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func GetInvalidation(ctx context.Context, infra Outputs, invalidationId string) (*cftypes.Invalidation, error) {
	cfClient := nsaws.NewCloudfrontClient(infra.Deployer, infra.Region)
	cdnId := ""
	if len(infra.CdnIds) > 0 {
		cdnId = infra.CdnIds[0]
	}
	out, err := cfClient.GetInvalidation(ctx, &cloudfront.GetInvalidationInput{
		DistributionId: aws.String(cdnId),
		Id:             aws.String(invalidationId),
	})
	if err != nil {
		return nil, err
	} else if out != nil {
		return out.Invalidation, nil
	}
	return nil, nil
}
