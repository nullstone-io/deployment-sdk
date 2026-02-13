package gcs

import (
	"context"
	"errors"
	"fmt"

	control "cloud.google.com/go/storage/control/apiv2"
	"cloud.google.com/go/storage/control/apiv2/controlpb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	ReadScopes = []string{
		"https://www.googleapis.com/auth/devstorage.read_only",
	}
)

// ListDirs queries a gcs bucket for a listing of keys to determine root directories
func ListDirs(ctx context.Context, infra Outputs) ([]string, error) {
	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadScopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating token source from service account: %w", err)
	}

	client, err := control.NewStorageControlClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()

	result := make([]string, 0)

	it := client.ListFolders(ctx, &controlpb.ListFoldersRequest{
		Parent:    infra.ArtifactsBucketId,
		Delimiter: "/",
	})
	for {
		cur, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return result, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error listing bucket objects: %w", err)
		}

		result = append(result, cur.Name)
	}
}
