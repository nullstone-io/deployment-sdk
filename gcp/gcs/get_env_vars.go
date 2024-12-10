package gcs

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/api/option"
)

func GetEnvVars(ctx context.Context, infra Outputs) (map[string]string, error) {
	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadScopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()

	reader, err := client.Bucket(infra.ArtifactsBucketName).Object(infra.EnvVarsFilename).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("error opening env var file in gcs bucket: %w", err)
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	m := map[string]string{}
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("environment variables file is an invalid format: %w", err)
	}
	return m, nil
}
