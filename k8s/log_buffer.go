package k8s

import (
	"github.com/nullstone-io/deployment-sdk/app"
)

type LogBuffer interface {
	NewWriter(id string) LogBufferWriter
}

type LogBufferWriter interface {
	Write(app.LogMessage)
	Close()
}

var (
	_ LogBuffer       = &SimpleLogBuffer{}
	_ LogBufferWriter = &SimpleLogBufferWriter{}
)

type SimpleLogBuffer struct {
	Emitter app.LogEmitter
}

func (s *SimpleLogBuffer) NewWriter(id string) LogBufferWriter {
	return &SimpleLogBufferWriter{Emitter: s.Emitter}
}

type SimpleLogBufferWriter struct {
	Emitter app.LogEmitter
}

func (s *SimpleLogBufferWriter) Write(message app.LogMessage) {
	s.Emitter(message)
}

func (s *SimpleLogBufferWriter) Close() {}
