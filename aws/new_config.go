package nsaws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/logging"
	"os"
)

const (
	DefaultAwsRegion = "us-east-1"
	AwsTraceEnvVar   = "AWS_TRACE"
)

func NewConfig(credentialsProvider aws.CredentialsProvider, region string) aws.Config {
	awsConfig := aws.Config{}
	if os.Getenv(AwsTraceEnvVar) != "" {
		awsConfig.Logger = logging.NewStandardLogger(os.Stderr)
		awsConfig.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
	}
	awsConfig.Region = DefaultAwsRegion
	if region != "" {
		awsConfig.Region = region
	}
	awsConfig.Credentials = aws.NewCredentialsCache(credentialsProvider)
	return awsConfig
}
