package gcs

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// ListObjects queries a GCS bucket for a listing of object keys
func ListObjects(ctx context.Context, infra Outputs, prefix, matchGlob string) ([]string, error) {
	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadScopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating token source from service account: %w", err)
	}

	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()

	var result []string
	query := &storage.Query{
		Delimiter:                "/",
		IncludeFoldersAsPrefixes: true,
		Prefix:                   prefix,
		MatchGlob:                matchGlob,
	}
	it := client.Bucket(infra.ArtifactsBucketName).Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return result, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error listing bucket objects: %w", err)
		}
		result = append(result, attrs.Name)
	}
}
