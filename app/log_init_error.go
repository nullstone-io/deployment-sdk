package app

import (
	"bytes"
	"errors"
	"github.com/mitchellh/colorstring"
)

func NewLogInitError(msg string, err error) LogInitError {
	return LogInitError{
		Message: msg,
		Err:     err,
	}
}

type LogInitError struct {
	Message string
	Err     error
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

func AsLogInitError(err error) (LogInitError, bool) {
	var lie LogInitError
	if errors.As(err, &lie) {
		return lie, true
	}
	return lie, false
}
