package modules

import (
	"context"
	"fmt"
	"gopkg.in/nullstone-io/go-api-client.v0"
	"time"
)

func NextBuild(ctx context.Context, cfg api.Config, manifest *Manifest) (string, error) {
	version, err := NextPatch(ctx, cfg, manifest)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	buildMetadata := fmt.Sprintf("%d%02d%02d.%02d%02d%02d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	return fmt.Sprintf("%s+%s", version, buildMetadata), nil
}
