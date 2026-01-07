package gcs

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

var (
	ReadWriteScopes = []string{"https://www.googleapis.com/auth/devstorage.read_write"}
)

type Uploader struct {
	BucketName     string
	ObjectKeyFn    func(filename string) string
	OnObjectUpload func(objectKey string)
}

func (u *Uploader) UploadDir(ctx context.Context, infra Outputs, baseDir string, filepaths []string) error {
	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadWriteScopes...)
	if err != nil {
		return fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()
	bucket := client.Bucket(u.BucketName)
	for _, filename := range filepaths {
		if err := u.uploadOne(ctx, bucket, baseDir, filename); err != nil {
			return fmt.Errorf("error uploading %q: %w", filename, err)
		}
	}
	return nil
}

func (u *Uploader) UploadFile(ctx context.Context, infra Outputs, fullFilepath string) error {
	tokenSource, err := infra.Deployer.TokenSource(ctx, ReadWriteScopes...)
	if err != nil {
		return fmt.Errorf("error creating token source from service account: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("error creating google storage client: %w", err)
	}
	defer client.Close()
	bucket := client.Bucket(u.BucketName)

	dir, filename := filepath.Dir(fullFilepath), filepath.Base(fullFilepath)
	if err := u.uploadOne(ctx, bucket, dir, filename); err != nil {
		return fmt.Errorf("error uploading %q: %w", filename, err)
	}
	return nil
}

func (u *Uploader) uploadOne(ctx context.Context, bucket *storage.BucketHandle, baseDir string, filename string) error {
	objectKey := u.ObjectKeyFn(filename)

	localFilepath := filepath.Join(baseDir, filename)
	file, err := os.Open(localFilepath)
	if err != nil {
		return fmt.Errorf("error opening local file %q: %w", localFilepath, err)
	}
	defer file.Close()

	writer := bucket.Object(objectKey).NewWriter(ctx)
	writer.ContentType = mime.TypeByExtension(filepath.Ext(filename))
	if writer.ContentType == "" {
		writer.ContentType = "text/plain"
	}
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("error uploading file %q: %w", filename, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close file uploader %q: %w", filename, err)
	}
	if u.OnObjectUpload != nil {
		u.OnObjectUpload(objectKey)
	}
	return nil
}
