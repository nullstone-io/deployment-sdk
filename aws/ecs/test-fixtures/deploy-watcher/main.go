package main

import (
	"context"
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

	deployment, err := ecs.UpdateServiceTask(context.Background(), infra, taskDefArn)
	if err != nil {
		log.Fatalln(err)
	}

	err = watcher.Watch(ctx, *deployment.Id)
	if err != nil {
		log.Fatalln(err)
	}
}
