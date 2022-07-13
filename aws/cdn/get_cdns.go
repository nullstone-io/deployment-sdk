package cdn

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/nullstone-io/deployment-sdk/aws"
)

func GetCdns(ctx context.Context, infra Outputs) ([]*cloudfront.GetDistributionOutput, error) {
	cfClient := nsaws.NewCloudfrontClient(infra.Deployer, infra.Region)
	cdns := make([]*cloudfront.GetDistributionOutput, 0)
	for _, cdnId := range infra.CdnIds {
		out, err := cfClient.GetDistribution(ctx, &cloudfront.GetDistributionInput{Id: aws.String(cdnId)})
		if err != nil {
			return nil, fmt.Errorf("error getting distribution %q: %w", cdnId, err)
		}
		cdns = append(cdns, out)
	}
	return cdns, nil
}
