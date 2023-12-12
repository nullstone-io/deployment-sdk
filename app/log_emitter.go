package app

import (
	"github.com/fatih/color"
	"github.com/nullstone-io/deployment-sdk/display"
	"io"
	"time"
)

var (
	bold   = color.New(color.Bold)
	normal = color.New()
)

type LogEmitter func(message LogMessage)

type LogMessage struct {
	// SourceType refers to the platform/provider that stores the logs
	// Examples: `cloudwatch`, `k8s`
	SourceType string `json:"sourceType"`

	// Source is where the logs are stored
	// Cloudwatch: Cloudwatch Log Group
	// Kubernetes: <empty>
	Source string `json:"source"`

	// Stream refers to the name of the log stream
	// Cloudwatch: Cloudwatch Log Stream name
	// Kubernetes: `podName/containerName`
	Stream string `json:"stream"`

	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

func NewWriterLogEmitter(w io.Writer) LogEmitter {
	return func(message LogMessage) {
		bold.Fprintf(w, "[%s]", message.Stream)
		if !message.Timestamp.IsZero() {
			normal.Fprintf(w, " %s", display.FormatTime(message.Timestamp))
		}
		normal.Fprintf(w, " %s", message.Message)
		normal.Fprintln(w)
	}
}
