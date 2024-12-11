package gcs

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/api/option"
)

func PutEnvVars(ctx context.Context, infra Outputs, envVars map[string]string) error {
	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadWriteScopes...)
	if err != nil {
		return fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()

	writer := client.Bucket(infra.ArtifactsBucketName).Object(infra.EnvVarsFilename).NewWriter(ctx)
	defer writer.Close()

	writer.ContentType = "application/json"
	encoder := json.NewEncoder(writer)
	return encoder.Encode(envVars)
}
