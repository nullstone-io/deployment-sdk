package composer

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/nullstone-io/deployment-sdk/artifacts"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/nullstone-io/deployment-sdk/outputs"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var ReadWriteScopes = []string{"https://www.googleapis.com/auth/devstorage.read_write"}

func NewPusher(ctx context.Context, osWriters logging.OsWriters, source outputs.RetrieverSource, appDetails app.Details) (app.Pusher, error) {
	outs, err := outputs.Retrieve[Outputs](ctx, source, appDetails.Workspace, appDetails.WorkspaceConfig)
	if err != nil {
		return nil, err
	}
	outs.InitializeCreds(source, appDetails.Workspace)

	return &Pusher{
		OsWriters:  osWriters,
		Source:     source,
		Infra:      outs,
		AppDetails: appDetails,
	}, nil
}

// Pusher syncs DAG files from a local source directory to the Composer-managed GCS bucket.
//
// Composer serves DAGs from a fixed location (gs://<bucket>/dags), so files are synced directly to
// that prefix rather than to a versioned artifact key.
type Pusher struct {
	OsWriters  logging.OsWriters
	Source     outputs.RetrieverSource
	Infra      Outputs
	AppDetails app.Details
}

// Push performs a recursive sync of the source directory to the DAG prefix, equivalent to:
//
//	gcloud storage rsync --recursive --delete-unmatched-destination-objects <source> gs://<bucket>/<prefix>
//
// Every local file is uploaded, then any destination object under the DAG prefix that has no matching
// local file is deleted.
func (p Pusher) Push(ctx context.Context, source, version string) error {
	stderr := p.OsWriters.Stderr()

	if source == "" {
		return fmt.Errorf("no source specified, source directory is required to push")
	}

	bucket := dagBucket(p.Infra)
	if bucket == "" {
		return fmt.Errorf("this app is missing the DAG bucket output (dag_gcs_bucket/dag_gcs_prefix); unable to push")
	}
	prefix := dagObjectPrefix(p.Infra)

	filepaths, err := artifacts.WalkDir(source)
	if err != nil {
		return fmt.Errorf("error scanning source: %w", err)
	}

	client, err := p.newStorageClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	bkt := client.Bucket(bucket)

	fmt.Fprintf(stderr, "Syncing %s to gs://%s/%s...\n", source, bucket, prefix)

	// Upload every local file, recording the object keys we expect to exist in the destination.
	desired := make(map[string]struct{}, len(filepaths))
	for _, rel := range filepaths {
		objectKey := path.Join(prefix, filepath.ToSlash(rel))
		desired[objectKey] = struct{}{}
		if err := uploadOne(ctx, bkt, source, rel, objectKey); err != nil {
			return fmt.Errorf("error uploading %q: %w", rel, err)
		}
		fmt.Fprintf(stderr, "Uploaded %s\n", objectKey)
	}

	// Delete any destination object under the DAG prefix that no longer has a matching local file.
	if err := p.deleteUnmatched(ctx, bkt, prefix, desired); err != nil {
		return err
	}

	fmt.Fprintln(stderr, "Sync complete")
	return nil
}

func (p Pusher) deleteUnmatched(ctx context.Context, bkt *storage.BucketHandle, prefix string, desired map[string]struct{}) error {
	stderr := p.OsWriters.Stderr()

	// Scope the listing to the DAG prefix so we never touch sibling folders (e.g. data/, plugins/).
	listPrefix := prefix
	if listPrefix != "" {
		listPrefix += "/"
	}

	it := bkt.Objects(ctx, &storage.Query{Prefix: listPrefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}
		// Skip "directory" placeholder objects and anything we just uploaded.
		if strings.HasSuffix(attrs.Name, "/") {
			continue
		}
		if _, ok := desired[attrs.Name]; ok {
			continue
		}
		if err := bkt.Object(attrs.Name).Delete(ctx); err != nil {
			return fmt.Errorf("error deleting %q: %w", attrs.Name, err)
		}
		fmt.Fprintf(stderr, "Deleted %s\n", attrs.Name)
	}
	return nil
}

func (p Pusher) Pull(ctx context.Context, version string) error {
	stderr := p.OsWriters.Stderr()

	bucket := dagBucket(p.Infra)
	if bucket == "" {
		return fmt.Errorf("this app is missing the DAG bucket output (dag_gcs_bucket/dag_gcs_prefix); unable to pull")
	}
	prefix := dagObjectPrefix(p.Infra)
	localDir := fmt.Sprintf("./%s-%s-dags", p.AppDetails.App.Name, p.AppDetails.Env.Name)

	client, err := p.newStorageClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	bkt := client.Bucket(bucket)

	fmt.Fprintf(stderr, "Downloading gs://%s/%s to %s...\n", bucket, prefix, localDir)
	it := bkt.Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}
		// Skip "directory" placeholder objects.
		if strings.HasSuffix(attrs.Name, "/") {
			continue
		}
		if err := downloadOne(ctx, bkt, attrs.Name, prefix, localDir); err != nil {
			return fmt.Errorf("error downloading %q: %w", attrs.Name, err)
		}
	}

	fmt.Fprintln(stderr, "Download complete")
	return nil
}

// ListArtifactVersions returns no versions because Composer DAGs are synced to a fixed bucket location
// rather than stored as discrete, versioned artifacts.
func (p Pusher) ListArtifactVersions(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p Pusher) newStorageClient(ctx context.Context) (*storage.Client, error) {
	tokenSource, err := p.Infra.Pusher.TokenSource(ctx, ReadWriteScopes...)
	if err != nil {
		return nil, fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("error creating google storage client: %w", err)
	}
	return client, nil
}

func uploadOne(ctx context.Context, bucket *storage.BucketHandle, baseDir, rel, objectKey string) error {
	localFilepath := filepath.Join(baseDir, rel)
	file, err := os.Open(localFilepath)
	if err != nil {
		return fmt.Errorf("error opening local file %q: %w", localFilepath, err)
	}
	defer file.Close()

	writer := bucket.Object(objectKey).NewWriter(ctx)
	writer.ContentType = mime.TypeByExtension(filepath.Ext(rel))
	if writer.ContentType == "" {
		writer.ContentType = "text/plain"
	}
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("error uploading file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close file uploader: %w", err)
	}
	return nil
}

func downloadOne(ctx context.Context, bucket *storage.BucketHandle, objectName, prefix, localDir string) error {
	rel := strings.TrimPrefix(strings.TrimPrefix(objectName, prefix), "/")
	if rel == "" {
		return nil
	}
	localFilepath := filepath.Join(localDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(localFilepath), 0755); err != nil {
		return fmt.Errorf("error creating local directory: %w", err)
	}

	reader, err := bucket.Object(objectName).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error opening object reader: %w", err)
	}
	defer reader.Close()

	file, err := os.Create(localFilepath)
	if err != nil {
		return fmt.Errorf("error creating local file %q: %w", localFilepath, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("error writing local file: %w", err)
	}
	return nil
}
