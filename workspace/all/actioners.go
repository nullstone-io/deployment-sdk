package all

import (
	aws_ecs_ec2_provider "github.com/nullstone-io/deployment-sdk/app/container/aws-ecs-ec2"
	aws_ecs_fargate_provider "github.com/nullstone-io/deployment-sdk/app/container/aws-ecs-fargate"
	aws_eks_provider "github.com/nullstone-io/deployment-sdk/app/container/aws-eks"
	gcp_gke_service "github.com/nullstone-io/deployment-sdk/app/container/gcp-gke-service"
	"github.com/nullstone-io/deployment-sdk/aws/ecs"
	"github.com/nullstone-io/deployment-sdk/aws/eks"
	"github.com/nullstone-io/deployment-sdk/gcp/gke"
	"github.com/nullstone-io/deployment-sdk/workspace"
)

var (
	// Actioners is a factory for creating a new Actioner from a workspace
	// If the factory method returns an error, it is wrapped with ActionNotSupportedError
	Actioners = workspace.Actioners{
		aws_eks_provider.ModuleContractName:         eks.NewActioner,
		gcp_gke_service.ModuleContractName:          gke.NewActioner,
		aws_ecs_fargate_provider.ModuleContractName: ecs.NewActioner,
		aws_ecs_ec2_provider.ModuleContractName:     ecs.NewActioner,
	}
)
