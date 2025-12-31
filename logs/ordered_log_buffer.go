package logs

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
)

// OrderedLogBuffer provides a way to stream several log sources in timestamp order
// This addresses an issue where streaming from several sources can result in out-of-order messages delivered to the user.
// This works by tracking a slot for each log source. (each log source is expected to be in timestamp order)
// As messages stream from those log sources, this buffer chooses the oldest message to emit first.
type OrderedLogBuffer struct {
	mu         sync.Mutex
	slots      map[string]*LogSlot
	pq         *priorityQueue
	output     chan<- app.LogMessage
	bufferSize int
	ctx        context.Context
}

// NewOrderedLogBuffer creates a new OrderedLogBuffer with the specified output channel and buffer size
func NewOrderedLogBuffer(ctx context.Context, output chan<- app.LogMessage, bufferSize int) *OrderedLogBuffer {
	return &OrderedLogBuffer{
		slots:      make(map[string]*LogSlot),
		pq:         &priorityQueue{},
		output:     output,
		bufferSize: bufferSize,
		ctx:        ctx,
	}
}

// ReserveSlot reserves a slot for a log source
func (b *OrderedLogBuffer) ReserveSlot(id string) *LogSlot {
	b.mu.Lock()
	defer b.mu.Unlock()

	if slot, exists := b.slots[id]; exists {
		return slot
	}

	slot := &LogSlot{
		id:      id,
		buffer:  make(chan app.LogMessage, b.bufferSize),
		release: make(chan struct{}),
		closed:  false,
		parent:  b,
	}

	b.slots[id] = slot
	go b.processSlot(slot)

	return slot
}

func (b *OrderedLogBuffer) releaseSlot(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.slots, id)
}

// processSlot processes messages from a single slot
func (b *OrderedLogBuffer) processSlot(slot *LogSlot) {
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-slot.release:
			return
		case msg := <-slot.buffer:
			b.mu.Lock()
			heap.Push(b.pq, &priorityQueueItem{
				message:   msg,
				timestamp: time.Now(),
				slotID:    slot.id,
			})
			b.emitReadyMessages()
			b.mu.Unlock()
		}
	}
}

// emitReadyMessages emits messages in order from the priority queue
func (b *OrderedLogBuffer) emitReadyMessages() {
	for b.pq.Len() > 0 {
		oldest := b.pq.Peek().(*priorityQueueItem)
		if slot, exists := b.slots[oldest.slotID]; exists && !slot.closed {
			// If the slot is still active, we might get older messages
			// So we only emit if this is the oldest message
			if b.pq.Len() == 1 || b.pq.Peek().(*priorityQueueItem).timestamp.Before(time.Now().Add(-100*time.Millisecond)) {
				item := heap.Pop(b.pq).(*priorityQueueItem)
				select {
				case b.output <- item.message:
				case <-b.ctx.Done():
					return
				}
			} else {
				break
			}
		} else {
			// Slot was released, remove all its messages
			heap.Pop(b.pq)
		}
	}
}

// LogSlot represents a single log source's slot in the buffer
type LogSlot struct {
	id      string
	buffer  chan app.LogMessage
	release chan struct{}
	closed  bool
	parent  *OrderedLogBuffer
}

// Emit sends a log message to the buffer
func (s *LogSlot) Emit(msg app.LogMessage) {
	if s.closed {
		return
	}
	select {
	case s.buffer <- msg:
	case <-s.parent.ctx.Done():
	}
}

// Release releases the slot and stops tracking its messages
func (s *LogSlot) Release() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.release)
	s.parent.releaseSlot(s.id)
}
