package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/ecs"
	"github.com/nullstone-io/deployment-sdk/logging"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// This is a utility to generate test fixtures for an ECS deployment

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()

	clusterArn := ""
	svcName := ""
	taskDefArn := ""

	infra := ecs.Outputs{
		Region:            "us-east-2",
		ServiceName:       svcName,
		TaskArn:           taskDefArn,
		MainContainerName: "main",
		Deployer: nsaws.User{
			Name:            "",
			AccessKeyId:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		},
		ClusterNamespace: ecs.ClusterNamespaceOutputs{ClusterArn: clusterArn},
	}
	osWriters := logging.StandardOsWriters{}
	watcher := app.PollingDeployWatcher{
		OsWriters: osWriters,
		StatusGetter: &ecs.DeployLogger{
			OsWriters: osWriters,
			Infra:     infra,
		},
	}

	newTaskDefArn := updateTaskDef(ctx, infra, taskDefArn, func(taskDef types.TaskDefinition) types.TaskDefinition {
		// NOTE: Use this area to update the task def to try failure scenarios

		//curImg := *taskDef.ContainerDefinitions[0].Image
		//badImg := fmt.Sprintf("%sxyz", curImg)
		//taskDef.ContainerDefinitions[0].Image = aws.String(badImg)

		return taskDef
	})

	deployment, err := ecs.UpdateServiceTask(context.Background(), infra, newTaskDefArn)
	if err != nil {
		log.Fatalln(err)
	}

	err = watcher.Watch(ctx, *deployment.Id, false)
	if err != nil {
		log.Fatalln(err)
	}
}

func updateTaskDef(ctx context.Context, infra ecs.Outputs, taskDefArn string, fn func(taskDef types.TaskDefinition) types.TaskDefinition) string {
	taskDef, err := ecs.GetTaskDefinitionByArn(ctx, infra, taskDefArn)
	if err != nil {
		log.Fatalln(err)
	} else if taskDef == nil {
		log.Fatalln("Cannot find task definition")
	}

	updatedTaskDef := fn(*taskDef)

	newTaskDef, err := ecs.UpdateTask(ctx, infra, &updatedTaskDef, *taskDef.TaskDefinitionArn)
	if err != nil {
		log.Fatalln(err)
	}
	return *newTaskDef.TaskDefinitionArn
}
