package lambda

import "github.com/aws/aws-sdk-go-v2/aws"

type Outputs interface {
	FunctionName() string
	DeployerAwsConfig() aws.Config
}
