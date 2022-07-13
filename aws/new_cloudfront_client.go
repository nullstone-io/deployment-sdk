package nsaws

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
)

func NewCloudfrontClient(user User, region string) *cloudfront.Client {
	cfg := NewConfig(user, region)
	opts := cloudfront.Options{
		Region:        cfg.Region,
		HTTPClient:    cfg.HTTPClient,
		Credentials:   cfg.Credentials,
		APIOptions:    cfg.APIOptions,
		Logger:        cfg.Logger,
		ClientLogMode: cfg.ClientLogMode,
	}
	if cfg.Retryer != nil {
		opts.Retryer = cfg.Retryer()
	}
	return cloudfront.New(opts)
}
