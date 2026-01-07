package gcs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func UploadDirArtifact(ctx context.Context, infra Outputs, source string, filepaths []string, version string) error {
	objDir := infra.ArtifactsKey(version)
	objDir = strings.TrimPrefix(objDir, "/")

	logger := log.New(os.Stderr, "", 0)
	uploader := Uploader{
		BucketName: infra.ArtifactsBucketName,
		ObjectKeyFn: func(filename string) string {
			objectKey := strings.Replace(filename, string(filepath.Separator), "/", -1)
			if objDir != "" {
				objectKey = path.Join(objDir, objectKey)
			}
			return objectKey
		},
		OnObjectUpload: func(objectKey string) {
			logger.Println(fmt.Sprintf("Uploaded %s", objectKey))
		},
	}
	return uploader.UploadDir(ctx, infra, source, filepaths)
}
