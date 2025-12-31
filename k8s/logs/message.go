package logs

import (
	"fmt"
	"strings"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
)

func MessageFromLine(appNamespace, appName, podName, containerName, line string) app.LogMessage {
	var ts time.Time
	timestamp, remaining := cutTimestampPrefix(line)
	if timestamp != nil {
		ts = *timestamp
	}

	return app.LogMessage{
		SourceType: "k8s",
		Source:     fmt.Sprintf("%s/%s", appNamespace, appName),
		Stream:     fmt.Sprintf("%s/%s", podName, containerName),
		Timestamp:  ts,
		Message:    remaining,
	}
}

func cutTimestampPrefix(line string) (*time.Time, string) {
	if before, after, found := strings.Cut(line, " "); found {
		if ts, parseErr := time.Parse(time.RFC3339, before); parseErr == nil {
			return &ts, after
		}
	}
	return nil, line
}
