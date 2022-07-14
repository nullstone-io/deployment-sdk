package logging

import "io"

// OsWriters contains two io.Writers that are used to write to stdout/stderr
type OsWriters interface {
	Stdout() io.Writer
	Stderr() io.Writer
}
