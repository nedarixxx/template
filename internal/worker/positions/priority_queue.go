package positions

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type Task struct {
	TripID      uuid.UUID
	DriverID    uuid.UUID
	StaleAt     time.Time
	SpanContext trace.SpanContext
}

type PushResult int

const (
	ResultQueued PushResult = iota
	ResultDuplicate
	ResultDropped
	ResultEvicted
)

type queueItem struct {
	task  Task
	index int
}

type itemMinHeap []*queueItem

func (h itemMinHeap) Len() int           { return len(h) }
func (h itemMinHeap) Less(i, j int) bool { return h[i].task.StaleAt.Before(h[j].task.StaleAt) }
func (h itemMinHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *itemMinHeap) Push(x any) {
	n := len(*h)
	item := x.(*queueItem)
	item.index = n
	*h = append(*h, item)
}

func (h *itemMinHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

func (h itemMinHeap) findFreshestIndex() int {
	if len(h) == 0 {
		return -1
	}
	start := len(h) / 2
	maxIdx := start
	maxTime := h[start].task.StaleAt
	for i := start + 1; i < len(h); i++ {
		if h[i].task.StaleAt.After(maxTime) {
			maxTime = h[i].task.StaleAt
			maxIdx = i
		}
	}
	return maxIdx
}

type PriorityQueue struct {
	mu       sync.Mutex
	capacity int
	items    itemMinHeap
	inQueue  map[uuid.UUID]*queueItem
	notify   chan struct{}
	closedCh chan struct{}
	closed   bool
}

func NewPriorityQueue(capacity int) *PriorityQueue {
	if capacity <= 0 {
		capacity = 100
	}
	return &PriorityQueue{
		capacity: capacity,
		items:    make(itemMinHeap, 0, capacity),
		inQueue:  make(map[uuid.UUID]*queueItem, capacity),
		notify:   make(chan struct{}, capacity),
		closedCh: make(chan struct{}),
	}
}

func (q *PriorityQueue) Push(task Task) (PushResult, *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ResultDropped, nil
	}

	if _, exists := q.inQueue[task.TripID]; exists {
		return ResultDuplicate, nil
	}

	newItem := &queueItem{task: task}

	if len(q.items) < q.capacity {
		heap.Push(&q.items, newItem)
		q.inQueue[task.TripID] = newItem
		select {
		case q.notify <- struct{}{}:
		default:
		}
		return ResultQueued, nil
	}

	freshestIdx := q.items.findFreshestIndex()
	if freshestIdx >= 0 && task.StaleAt.Before(q.items[freshestIdx].task.StaleAt) {
		evictedItem := heap.Remove(&q.items, freshestIdx).(*queueItem)
		delete(q.inQueue, evictedItem.task.TripID)

		heap.Push(&q.items, newItem)
		q.inQueue[task.TripID] = newItem

		evictedTask := evictedItem.task
		return ResultEvicted, &evictedTask
	}

	return ResultDropped, nil
}

func (q *PriorityQueue) Pop(ctx context.Context) (Task, bool) {
	for {
		select {
		case <-ctx.Done():
			return Task{}, false
		case <-q.notify:
			q.mu.Lock()
			if len(q.items) == 0 {
				closed := q.closed
				q.mu.Unlock()
				if closed {
					return Task{}, false
				}
				continue
			}
			item := heap.Pop(&q.items).(*queueItem)
			task := item.task
			q.mu.Unlock()
			return task, true
		case <-q.closedCh:
			q.mu.Lock()
			if len(q.items) == 0 {
				q.mu.Unlock()
				return Task{}, false
			}
			item := heap.Pop(&q.items).(*queueItem)
			task := item.task
			q.mu.Unlock()
			return task, true
		}
	}
}

func (q *PriorityQueue) Cancel(tripID uuid.UUID) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	item, exists := q.inQueue[tripID]
	if !exists {
		return false
	}

	wasInHeap := false
	if item.index >= 0 {
		heap.Remove(&q.items, item.index)
		wasInHeap = true
	}
	delete(q.inQueue, tripID)
	return wasInHeap
}

func (q *PriorityQueue) IsActive(tripID uuid.UUID) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, exists := q.inQueue[tripID]
	return exists
}

func (q *PriorityQueue) MarkDone(tripID uuid.UUID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.inQueue, tripID)
}

func (q *PriorityQueue) CloseInput() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.closedCh)
	}
}

func (q *PriorityQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
