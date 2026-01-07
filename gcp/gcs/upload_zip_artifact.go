package gcs

import (
	"context"
	"fmt"
	"log"
	"os"
)

func UploadZipArtifact(ctx context.Context, infra Outputs, source string, version string) error {
	objKey := infra.ArtifactsKey(version)

	logger := log.New(os.Stderr, "", 0)
	uploader := Uploader{
		BucketName: infra.ArtifactsBucketName,
		ObjectKeyFn: func(filename string) string {
			return objKey
		},
		OnObjectUpload: func(objectKey string) {
			logger.Println(fmt.Sprintf("Uploaded %s", objectKey))
		},
	}
	return uploader.UploadFile(ctx, infra, source)
}
