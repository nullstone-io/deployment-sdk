package logging

import (
	"context"
	"io"
)

// OsWriters contains two io.Writers that are used to write to stdout/stderr
type OsWriters interface {
	Stdout() io.Writer
	Stderr() io.Writer
}

type contextKey struct{}

func OsWritersFromContext(ctx context.Context) OsWriters {
	if val, ok := ctx.Value(contextKey{}).(OsWriters); ok {
		return val
	}
	return nil
}

func ContextWithOsWriters(ctx context.Context, osWriters OsWriters) context.Context {
	return context.WithValue(ctx, contextKey{}, osWriters)
}
