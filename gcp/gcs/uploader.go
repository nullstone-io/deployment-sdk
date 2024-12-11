package gcs

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ReadWriteScopes = []string{"https://www.googleapis.com/auth/devstorage.read_write"}
)

type Uploader struct {
	BucketName      string
	ObjectDirectory string
	OnObjectUpload  func(objectKey string)
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
	for _, fp := range filepaths {
		if err := u.uploadOne(ctx, bucket, baseDir, fp); err != nil {
			return fmt.Errorf("error uploading %q: %w", fp, err)
		}
	}
	return nil
}

func (u *Uploader) uploadOne(ctx context.Context, bucket *storage.BucketHandle, baseDir string, fp string) error {
	localFilepath := filepath.Join(baseDir, fp)
	file, err := os.Open(localFilepath)
	if err != nil {
		return fmt.Errorf("error opening local file %q: %w", localFilepath, err)
	}
	defer file.Close()

	objectKey := strings.Replace(fp, string(filepath.Separator), "/", -1)
	if u.ObjectDirectory != "" {
		objectKey = path.Join(u.ObjectDirectory, objectKey)
	}

	writer := bucket.Object(objectKey).NewWriter(ctx)
	writer.ContentType = mime.TypeByExtension(filepath.Ext(fp))
	if writer.ContentType == "" {
		writer.ContentType = "text/plain"
	}
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("error uploading file %q: %w", fp, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close file uploader %q: %w", fp, err)
	}
	if u.OnObjectUpload != nil {
		u.OnObjectUpload(objectKey)
	}
	return nil
}
