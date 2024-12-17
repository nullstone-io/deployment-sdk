package k8s

import (
	"bytes"
	"github.com/nullstone-io/deployment-sdk/display"
	"strings"
	"time"
)

const (
	EventTypeNormal  = "Normal"
	EventTypeWarning = "Warning"
)

type DeployEvent struct {
	Timestamp time.Time
	Type      string
	Reason    string
	Object    string
	Message   string
}

func (e DeployEvent) String() string {
	buf := bytes.NewBufferString(display.FormatTime(e.Timestamp))
	buf.WriteString(" [")
	buf.WriteString(e.Object)
	buf.WriteString("]")
	if len(e.Object) < 32 {
		buf.WriteString(strings.Repeat(" ", 32-len(e.Object)))
	}
	buf.WriteString(" (")
	buf.WriteString(e.Reason)
	buf.WriteString(") ")
	if e.Type == EventTypeWarning {
		buf.WriteString("[yellow]")
	}
	buf.WriteString(e.Message)
	if e.Type == EventTypeWarning {
		buf.WriteString("[reset]")
	}
	return buf.String()
}
