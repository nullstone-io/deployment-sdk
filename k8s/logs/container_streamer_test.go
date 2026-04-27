package logs

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type recordingBuffer struct {
	mu       sync.Mutex
	messages []app.LogMessage
}

func (b *recordingBuffer) NewWriter(string) BufferWriter { return &recordingWriter{buf: b} }

func (b *recordingBuffer) lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.messages))
	for i, m := range b.messages {
		out[i] = m.Message
	}
	return out
}

type recordingWriter struct{ buf *recordingBuffer }

func (w *recordingWriter) Write(m app.LogMessage) {
	w.buf.mu.Lock()
	defer w.buf.mu.Unlock()
	w.buf.messages = append(w.buf.messages, m)
}
func (w *recordingWriter) Close() {}

// fakeLogSource hands out pre-made io.ReadClosers on each call to Stream.
// The test controls exactly what bytes appear on each successive Stream() call.
type fakeLogSource struct {
	streams []io.ReadCloser
	idx     int
	mu      sync.Mutex
}

var _ StreamGetter = (*fakeLogSource)(nil)
var _ rest.ResponseWrapper = (*fakeLogSource)(nil)

func (f *fakeLogSource) GetStreamer(*corev1.Pod, string, time.Time, bool) (rest.ResponseWrapper, error) {
	return f, nil
}

func (f *fakeLogSource) DoRaw(context.Context) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeLogSource) Stream(ctx context.Context) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.idx >= len(f.streams) {
		return nil, fmt.Errorf("fakeLogSource: no more streams (idx=%d)", f.idx)
	}
	rc := f.streams[f.idx]
	f.idx++
	return rc, nil
}

func testPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod-x"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}}},
	}
}

func flushTimeout(d time.Duration) *time.Duration { return &d }

// Reproduces NUL-25: the initial follow stream EOFs (kubelet closed it on
// container exit) before the caller signals Stop. Prior to the fix, the
// streamer returned with no flush, dropping any bytes that reached the k8s
// API on a fresh stream. This test writes the tail line only into the
// flush-stage stream — if flush-on-EOF isn't wired up, the tail is lost.
func TestContainerStreamer_FlushRunsWhenInitialStreamEOFs(t *testing.T) {
	mainR, mainW := io.Pipe()
	flushR, flushW := io.Pipe()
	src := &fakeLogSource{streams: []io.ReadCloser{mainR, flushR}}

	go func() {
		_, _ = mainW.Write([]byte("line1\nline2\n"))
		_ = mainW.Close()
	}()
	go func() {
		_, _ = flushW.Write([]byte("tail-line\n"))
		_ = flushW.Close()
	}()

	streamer := &ContainerStreamer{
		Namespace:     "ns",
		WorkloadName:  "wl",
		Pod:           testPod(),
		ContainerName: "main",
		LogSource:     src,
	}
	buf := &recordingBuffer{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamer.Stream(context.Background(), app.LogStreamOptions{
			StopFlushTimeout: flushTimeout(2 * time.Second),
		}, buf)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not exit")
	}

	got := buf.lines()
	want := []string{"line1", "line2", "tail-line"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

// The idle-window short-circuit must let flush exit well before the
// StopFlushTimeout cap when no more bytes are arriving. Validates the
// "2s job shouldn't take 10s" acceptance criterion.
func TestContainerStreamer_FlushExitsOnIdle(t *testing.T) {
	mainR, mainW := io.Pipe()
	flushR, flushW := io.Pipe()
	src := &fakeLogSource{streams: []io.ReadCloser{mainR, flushR}}

	go func() {
		_, _ = mainW.Write([]byte("line1\n"))
		_ = mainW.Close()
	}()
	// flushW writes one line then goes idle (no close, no more writes).
	// The idle-window (250ms) should fire and let flush return.
	go func() {
		_, _ = flushW.Write([]byte("tail-line\n"))
	}()

	streamer := &ContainerStreamer{
		Namespace:     "ns",
		WorkloadName:  "wl",
		Pod:           testPod(),
		ContainerName: "main",
		LogSource:     src,
	}
	buf := &recordingBuffer{}

	start := time.Now()
	// 5s cap — idle-window should make this return in well under that.
	streamer.Stream(context.Background(), app.LogStreamOptions{
		StopFlushTimeout: flushTimeout(5 * time.Second),
	}, buf)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("flush did not short-circuit on idle: took %s", elapsed)
	}
	lines := buf.lines()
	if len(lines) != 2 || lines[0] != "line1" || lines[1] != "tail-line" {
		t.Fatalf("unexpected lines %q", lines)
	}
	_ = flushW.Close()
}

// StopFlushTimeout=0 (or nil) keeps flush as a no-op — important for callers
// that opt out of flushing entirely.
func TestContainerStreamer_NoFlushWhenTimeoutZero(t *testing.T) {
	mainR, mainW := io.Pipe()
	// Only one stream configured — if flush tried to open a second one,
	// fakeLogSource.Stream returns an error and debug-logs; we just assert
	// the main-stream bytes came through.
	src := &fakeLogSource{streams: []io.ReadCloser{mainR}}

	go func() {
		_, _ = mainW.Write([]byte("only-line\n"))
		_ = mainW.Close()
	}()

	streamer := &ContainerStreamer{
		Namespace:     "ns",
		WorkloadName:  "wl",
		Pod:           testPod(),
		ContainerName: "main",
		LogSource:     src,
	}
	buf := &recordingBuffer{}
	streamer.Stream(context.Background(), app.LogStreamOptions{}, buf)

	lines := buf.lines()
	if len(lines) != 1 || lines[0] != "only-line" {
		t.Fatalf("unexpected lines %q", lines)
	}
	if src.idx != 1 {
		t.Fatalf("flush should not have opened a second stream; idx=%d", src.idx)
	}
}
