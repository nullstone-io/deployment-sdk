package logs

import (
	"time"

	"github.com/nullstone-io/deployment-sdk/app"
)

// Priority queue implementation
type priorityQueueItem struct {
	message   app.LogMessage
	timestamp time.Time
	slotID    string
	index     int
}

type priorityQueue []*priorityQueueItem

func (q *priorityQueue) Len() int { return len(*q) }

func (q *priorityQueue) Less(i, j int) bool {
	return (*q)[i].timestamp.Before((*q)[j].timestamp)
}

func (q *priorityQueue) Swap(i, j int) {
	(*q)[i], (*q)[j] = (*q)[j], (*q)[i]
	(*q)[i].index = i
	(*q)[j].index = j
}

func (q *priorityQueue) Push(x any) {
	n := len(*q)
	item := x.(*priorityQueueItem)
	item.index = n
	*q = append(*q, item)
}

func (q *priorityQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*q = old[0 : n-1]
	return item
}

func (q *priorityQueue) Peek() any {
	if len(*q) == 0 {
		return nil
	}
	return (*q)[0]
}
