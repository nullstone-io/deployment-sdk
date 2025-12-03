package all

import (
	"github.com/nullstone-io/deployment-sdk/app"
	aws_batch_fargate "github.com/nullstone-io/deployment-sdk/app/container/aws-batch-fargate"
	"github.com/nullstone-io/deployment-sdk/app/container/aws-ecs-ec2"
	"github.com/nullstone-io/deployment-sdk/app/container/aws-ecs-fargate"
	gcp_cloudrun "github.com/nullstone-io/deployment-sdk/app/container/gcp-cloudrun"
	gcp_gke_service "github.com/nullstone-io/deployment-sdk/app/container/gcp-gke-service"
	"github.com/nullstone-io/deployment-sdk/app/server/aws-beanstalk"
	"github.com/nullstone-io/deployment-sdk/app/serverless/aws-lambda-container"
	"github.com/nullstone-io/deployment-sdk/app/serverless/aws-lambda-zip"
	"github.com/nullstone-io/deployment-sdk/app/static-site/aws-s3"
	"github.com/nullstone-io/deployment-sdk/app/static-site/gcp-gcs"
)

var (
	Providers = app.Providers{
		aws_batch_fargate.ModuleContractName:    aws_batch_fargate.Provider,
		aws_ecs_fargate.ModuleContractName:      aws_ecs_fargate.Provider,
		aws_ecs_ec2.ModuleContractName:          aws_ecs_ec2.Provider,
		aws_s3.ModuleContractName:               aws_s3.Provider,
		aws_lambda_zip.ModuleContractName:       aws_lambda_zip.Provider,
		aws_lambda_container.ModuleContractName: aws_lambda_container.Provider,
		aws_beanstalk.ModuleContractName:        aws_beanstalk.Provider,
		gcp_gke_service.ModuleContractName:      gcp_gke_service.Provider,
		gcp_gcs.ModuleContractName:              gcp_gcs.Provider,
		gcp_cloudrun.ModuleContractName:         gcp_cloudrun.Provider,
	}
)
