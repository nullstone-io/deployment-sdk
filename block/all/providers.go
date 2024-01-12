package all

import (
	"github.com/nullstone-io/deployment-sdk/block"
	aws_beanstalk "github.com/nullstone-io/deployment-sdk/block/aws-beanstalk"
	aws_ecs_ec2 "github.com/nullstone-io/deployment-sdk/block/aws-ecs-ec2"
	aws_ecs_fargate "github.com/nullstone-io/deployment-sdk/block/aws-ecs-fargate"
	aws_lambda_container "github.com/nullstone-io/deployment-sdk/block/aws-lambda-container"
	aws_lambda_zip "github.com/nullstone-io/deployment-sdk/block/aws-lambda-zip"
	gcp_gke_service "github.com/nullstone-io/deployment-sdk/block/gcp-gke-service"
)

var (
	Providers = block.Providers{
		aws_ecs_fargate.ModuleContractName:      aws_ecs_fargate.Provider,
		aws_ecs_ec2.ModuleContractName:          aws_ecs_ec2.Provider,
		aws_lambda_zip.ModuleContractName:       aws_lambda_zip.Provider,
		aws_lambda_container.ModuleContractName: aws_lambda_container.Provider,
		aws_beanstalk.ModuleContractName:        aws_beanstalk.Provider,
		gcp_gke_service.ModuleContractName:      gcp_gke_service.Provider,
	}
)
