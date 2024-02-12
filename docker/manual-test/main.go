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

	pusher := ecr.Pusher{
		OsWriters: logging.StandardOsWriters{},
		Infra: ecr.Outputs{
			Region:       "us-east-1",
			ImageRepoUrl: docker.ParseImageUrl("820877947822.dkr.ecr.us-east-1.amazonaws.com/api-gateway-bufms"),
			ImagePusher: nsaws.User{
				Name:            "image-pusher-api-gateway-bufms",
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
