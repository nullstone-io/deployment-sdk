package logs

import "github.com/nullstone-io/deployment-sdk/app"

type Buffer interface {
	NewWriter(id string) BufferWriter
}

type BufferWriter interface {
	Write(app.LogMessage)
	Close()
}

var (
	_ Buffer       = &SimpleLogBuffer{}
	_ BufferWriter = &SimpleBufferWriter{}
)

type SimpleLogBuffer struct {
	Emitter app.LogEmitter
}

func (s *SimpleLogBuffer) NewWriter(id string) BufferWriter {
	return &SimpleBufferWriter{Emitter: s.Emitter}
}

type SimpleBufferWriter struct {
	Emitter app.LogEmitter
}

func (s *SimpleBufferWriter) Write(message app.LogMessage) {
	s.Emitter(message)
}

func (s *SimpleBufferWriter) Close() {}
