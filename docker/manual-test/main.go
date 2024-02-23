package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nullstone-io/deployment-sdk/aws"
	"github.com/nullstone-io/deployment-sdk/aws/ecr"
	"github.com/nullstone-io/deployment-sdk/docker"
	"github.com/nullstone-io/deployment-sdk/logging"
	"log"
	"os"
)

func main() {
	ctx := context.Background()

	accountId := "" // AWS Account ID
	region := "us-east-1"
	repoName := "" // ECR image repo name
	imageUrl := docker.ParseImageUrl(fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s", accountId, region, repoName))

	pusher := ecr.Pusher{
		OsWriters: logging.StandardOsWriters{},
		Infra: ecr.Outputs{
			Region:       region,
			ImageRepoUrl: imageUrl,
			ImagePusher: nsaws.User{
				AccessKeyId:     os.Getenv("AWS_ACCESS_KEY_ID"),
				SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			},
		},
	}

	tags, err := pusher.ListArtifactVersions(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	raw, err := json.MarshalIndent(tags, "", "  ")
	fmt.Println(string(raw))
}
