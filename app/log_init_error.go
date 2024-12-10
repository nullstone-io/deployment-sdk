package app

import (
	"bytes"
	"errors"
	"github.com/mitchellh/colorstring"
	"time"
)

func NewLogInitError(sourceType, source, msg string, err error) LogInitError {
	return LogInitError{
		SourceType: sourceType,
		Source:     source,
		Timestamp:  time.Now(),
		Message:    msg,
		Err:        err,
	}
}

type LogInitError struct {
	// SourceType refers to the platform/provider that stores the logs
	// Examples: `cloudwatch`, `k8s`
	SourceType string `json:"sourceType"`

	// Source is where the logs are stored
	// Cloudwatch: Cloudwatch Log Group
	// Kubernetes: `appNamespace/appName`
	Source string `json:"source"`

	Timestamp time.Time

	// Message is a pretty message produced by Nullstone based on the context of where the error occurred
	Message string

	// Err is the source's error message
	Err error
}

func (e LogInitError) Error() string {
	buf := bytes.NewBufferString("")
	if e.Err != nil {
		colorstring.Fprintf(buf, "[red]%s: %s[reset]", e.Message, e.Err.Error())
	} else {
		colorstring.Fprintf(buf, "[red]%s[reset]", e.Message)
	}
	return buf.String()
}

func (e LogInitError) LogMessage() LogMessage {
	return LogMessage{
		SourceType: e.SourceType,
		Source:     e.Source,
		Stream:     "",
		Timestamp:  e.Timestamp,
		Message:    e.Error(),
	}
}

func AsLogInitError(err error) (LogInitError, bool) {
	var lie LogInitError
	if errors.As(err, &lie) {
		return lie, true
	}
	return lie, false
}
