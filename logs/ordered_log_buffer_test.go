package logs

import (
	"context"
	"testing"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderedLogBuffer(t *testing.T) {
	t.Run("emits messages in order", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		output := make(chan app.LogMessage, 10)
		buf := NewOrderedLogBuffer(ctx, output, 10)

		// Create two slots
		slot1 := buf.ReserveSlot("slot1")
		slot2 := buf.ReserveSlot("slot2")

		// Send messages out of order
		now := time.Now()
		slot2.Emit(app.LogMessage{Message: "slot2-1", Timestamp: now})
		slot1.Emit(app.LogMessage{Message: "slot1-1", Timestamp: now.Add(-time.Second)}) // Earlier timestamp
		slot2.Emit(app.LogMessage{Message: "slot2-2", Timestamp: now.Add(time.Second)})

		// Give the buffer time to process
		time.Sleep(100 * time.Millisecond)

		// Should receive messages in timestamp order
		require.Equal(t, 3, len(output))
		got := make([]string, 0)
		msg1 := <-output
		got = append(got, msg1.Message)
		msg2 := <-output
		got = append(got, msg2.Message)
		msg3 := <-output
		got = append(got, msg3.Message)

		want := []string{
			"slot1-1",
			"slot2-1",
			"slot2-2",
		}
		assert.Equal(t, want, got)
	})

	t.Run("handles slot release", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		output := make(chan app.LogMessage, 10)
		buf := NewOrderedLogBuffer(ctx, output, 10)

		slot1 := buf.ReserveSlot("slot1")
		slot2 := buf.ReserveSlot("slot2")

		// Send messages
		slot1.Emit(app.LogMessage{Message: "slot1-1"})
		slot2.Emit(app.LogMessage{Message: "slot2-1"})
		slot1.Release() // Release slot1

		// Send more messages after release
		slot2.Emit(app.LogMessage{Message: "slot2-2"})
		slot1.Emit(app.LogMessage{Message: "slot1-2"}) // Should be ignored

		time.Sleep(100 * time.Millisecond)

		// Should only get messages before release and from slot2
		assert.Equal(t, 2, len(output))
		msg1 := <-output
		msg2 := <-output
		assert.Equal(t, "slot1-1", msg1.Message)
		assert.Equal(t, "slot2-1", msg2.Message)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		output := make(chan app.LogMessage, 10)
		ctx, cancel := context.WithCancel(context.Background())
		buf := NewOrderedLogBuffer(ctx, output, 10)

		slot := buf.ReserveSlot("slot1")
		slot.Emit(app.LogMessage{Message: "before"})

		// Cancel the context
		cancel()
		time.Sleep(10 * time.Millisecond) // Give time for cancellation to propagate

		// This message should be dropped
		slot.Emit(app.LogMessage{Message: "after"})

		// Only the first message should be received
		assert.Equal(t, 1, len(output))
		assert.Equal(t, "before", (<-output).Message)
	})

	t.Run("reuses existing slot", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		buf := NewOrderedLogBuffer(ctx, make(chan app.LogMessage), 10)

		slot1a := buf.ReserveSlot("slot1")
		slot1b := buf.ReserveSlot("slot1") // Should return the same slot

		// Both variables should point to the same slot
		assert.Equal(t, slot1a, slot1b)
	})

	t.Run("handles buffer overflow", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		output := make(chan app.LogMessage, 10)
		buf := NewOrderedLogBuffer(ctx, output, 2) // Small buffer size

		slot := buf.ReserveSlot("slot1")

		// Fill the buffer
		for i := 0; i < 10; i++ {
			slot.Emit(app.LogMessage{Message: "msg"})
		}

		// Should not block or panic
		select {
		case <-output:
			// At least one message should be processed
		case <-time.After(100 * time.Millisecond):
			t.Error("Expected messages to be processed")
		}
	})
}
