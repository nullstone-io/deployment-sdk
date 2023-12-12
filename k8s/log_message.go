package k8s

import (
	"fmt"
	"github.com/nullstone-io/deployment-sdk/app"
	"strings"
	"time"
)

func LogMessageFromLine(podName, containerName, line string) app.LogMessage {
	var ts time.Time
	timestamp, remaining := cutTimestampPrefix(line)
	if timestamp != nil {
		ts = *timestamp
	}

	return app.LogMessage{
		SourceType: "k8s",
		Source:     "",
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
