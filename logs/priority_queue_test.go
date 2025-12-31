package logs

import (
	"container/heap"
	"testing"
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
	"github.com/stretchr/testify/assert"
)

func TestPriorityQueue(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		pq := &priorityQueue{}
		heap.Init(pq)

		// Test empty queue
		assert.Equal(t, 0, pq.Len())
		assert.Nil(t, pq.Peek())

		// Test Push
		now := time.Now()
		items := []*priorityQueueItem{
			{message: app.LogMessage{Message: "msg3"}, timestamp: now.Add(3 * time.Second)},
			{message: app.LogMessage{Message: "msg1"}, timestamp: now},
			{message: app.LogMessage{Message: "msg2"}, timestamp: now.Add(2 * time.Second)},
		}

		for _, it := range items {
			heap.Push(pq, it)
		}

		// Test Len
		assert.Equal(t, 3, pq.Len())

		// Test Peek
		peeked := pq.Peek().(*priorityQueueItem)
		assert.Equal(t, "msg1", peeked.message.Message)

		// Test Pop order (should be in timestamp order)
		first := heap.Pop(pq).(*priorityQueueItem)
		assert.Equal(t, "msg1", first.message.Message)
		assert.Equal(t, 2, pq.Len())

		second := heap.Pop(pq).(*priorityQueueItem)
		assert.Equal(t, "msg2", second.message.Message)
		assert.Equal(t, 1, pq.Len())

		third := heap.Pop(pq).(*priorityQueueItem)
		assert.Equal(t, "msg3", third.message.Message)
		assert.Equal(t, 0, pq.Len())
		assert.Nil(t, pq.Peek())
	})

	t.Run("update priority", func(t *testing.T) {
		pq := &priorityQueue{}
		heap.Init(pq)

		now := time.Now()
		items := []*priorityQueueItem{
			{message: app.LogMessage{Message: "msg1"}, timestamp: now.Add(time.Second)},
			{message: app.LogMessage{Message: "msg2"}, timestamp: now.Add(2 * time.Second)},
		}

		for _, it := range items {
			heap.Push(pq, it)
		}

		// Update the timestamp of the first item to be the latest
		items[0].timestamp = now.Add(3 * time.Second)
		heap.Fix(pq, items[0].index)

		// The second item should now be first
		first := heap.Pop(pq).(*priorityQueueItem)
		assert.Equal(t, "msg2", first.message.Message)
	})

	t.Run("edge cases", func(t *testing.T) {
		pq := &priorityQueue{}
		heap.Init(pq)

		// Test empty Pop
		assert.Panics(t, func() {
			heap.Pop(pq)
		}, "should panic when popping from empty queue")

		// Test with single item
		heap.Push(pq, &priorityQueueItem{message: app.LogMessage{Message: "single"}})
		assert.Equal(t, 1, pq.Len())
		item := heap.Pop(pq).(*priorityQueueItem)
		assert.Equal(t, "single", item.message.Message)
	})

	t.Run("swap maintains indices", func(t *testing.T) {
		now := time.Now()
		itemA := &priorityQueueItem{index: 0, message: app.LogMessage{Message: "a"}, timestamp: now}
		itemB := &priorityQueueItem{index: 1, message: app.LogMessage{Message: "b"}, timestamp: now.Add(time.Second)}
		pq := priorityQueue{itemA, itemB}

		// Before swap
		assert.Equal(t, 0, pq[0].index)
		assert.Equal(t, 1, pq[1].index)

		pq.Swap(0, 1)

		// After swap
		assert.Equal(t, 1, itemA.index)
		assert.Equal(t, 0, itemB.index)
	})
}
