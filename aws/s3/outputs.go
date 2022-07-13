package s3

import (
	"github.com/nullstone-io/deployment-sdk/aws"
)

type Outputs struct {
	Region     string     `ns:"region"`
	BucketName string     `ns:"bucket_name"`
	BucketArn  string     `ns:"bucket_arn"`
	Deployer   nsaws.User `ns:"deployer"`
	CdnIds     []string   `ns:"cdn_ids"`
}
