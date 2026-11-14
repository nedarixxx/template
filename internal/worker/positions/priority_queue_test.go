package positions_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/worker/positions"
)

func TestPriorityQueue_Order(t *testing.T) {
	q := positions.NewPriorityQueue(10)
	now := time.Now().UTC()

	trip1 := positions.Task{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		StaleAt:  now.Add(-30 * time.Minute),
	}
	trip2 := positions.Task{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		StaleAt:  now.Add(-20 * time.Minute),
	}
	trip3 := positions.Task{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		StaleAt:  now.Add(-10 * time.Minute),
	}

	q.Push(trip2)
	q.Push(trip1)
	q.Push(trip3)

	ctx := context.Background()

	pop1, ok := q.Pop(ctx)
	if !ok || pop1.TripID != trip1.TripID {
		t.Fatalf("expected trip1 (oldest), got %v", pop1.TripID)
	}

	pop2, ok := q.Pop(ctx)
	if !ok || pop2.TripID != trip2.TripID {
		t.Fatalf("expected trip2, got %v", pop2.TripID)
	}

	pop3, ok := q.Pop(ctx)
	if !ok || pop3.TripID != trip3.TripID {
		t.Fatalf("expected trip3, got %v", pop3.TripID)
	}
}

func TestPriorityQueue_EvictionAndDropping(t *testing.T) {
	q := positions.NewPriorityQueue(2)
	now := time.Now().UTC()

	tripB := positions.Task{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		StaleAt:  now.Add(-20 * time.Minute),
	}
	tripC := positions.Task{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		StaleAt:  now.Add(-10 * time.Minute),
	}

	res, _ := q.Push(tripB)
	if res != positions.ResultQueued {
		t.Fatalf("expected ResultQueued, got %v", res)
	}
	res, _ = q.Push(tripC)
	if res != positions.ResultQueued {
		t.Fatalf("expected ResultQueued, got %v", res)
	}

	tripA := positions.Task{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		StaleAt:  now.Add(-30 * time.Minute),
	}

	res, evicted := q.Push(tripA)
	if res != positions.ResultEvicted {
		t.Fatalf("expected ResultEvicted, got %v", res)
	}
	if evicted == nil || evicted.TripID != tripC.TripID {
		t.Fatalf("expected tripC to be evicted, got %v", evicted)
	}

	tripD := positions.Task{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		StaleAt:  now.Add(-5 * time.Minute),
	}

	res, _ = q.Push(tripD)
	if res != positions.ResultDropped {
		t.Fatalf("expected ResultDropped, got %v", res)
	}

	ctx := context.Background()
	pop1, _ := q.Pop(ctx)
	if pop1.TripID != tripA.TripID {
		t.Fatalf("expected tripA, got %v", pop1.TripID)
	}
	pop2, _ := q.Pop(ctx)
	if pop2.TripID != tripB.TripID {
		t.Fatalf("expected tripB, got %v", pop2.TripID)
	}
}

func TestPriorityQueue_DedupAndCancel(t *testing.T) {
	q := positions.NewPriorityQueue(10)
	now := time.Now().UTC()
	tripID := uuid.New()

	task := positions.Task{
		TripID:   tripID,
		DriverID: uuid.New(),
		StaleAt:  now,
	}

	res, _ := q.Push(task)
	if res != positions.ResultQueued {
		t.Fatalf("expected ResultQueued, got %v", res)
	}

	res, _ = q.Push(task)
	if res != positions.ResultDuplicate {
		t.Fatalf("expected ResultDuplicate, got %v", res)
	}

	q.Cancel(tripID)

	if q.IsActive(tripID) {
		t.Fatalf("expected trip to be inactive after cancel")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, ok := q.Pop(ctx)
	if ok {
		t.Fatalf("expected Pop to fail on empty cancelled queue")
	}
}

func TestPriorityQueue_Concurrent(t *testing.T) {
	q := positions.NewPriorityQueue(50)
	now := time.Now().UTC()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				tID := uuid.New()
				q.Push(positions.Task{
					TripID:   tID,
					DriverID: uuid.New(),
					StaleAt:  now.Add(time.Duration(j-idx) * time.Minute),
				})
				if j%3 == 0 {
					q.Cancel(tID)
				}
			}
		}(i)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				task, ok := q.Pop(ctx)
				if !ok {
					return
				}
				q.MarkDone(task.TripID)
			}
		}()
	}

	wg.Wait()
	if q.Size() > 50 {
		t.Fatalf("expected size <= 50, got %d", q.Size())
	}
}

func TestPriorityQueue_CloseAndDrain(t *testing.T) {
	q := positions.NewPriorityQueue(5)
	now := time.Now().UTC()

	trip1 := positions.Task{TripID: uuid.New(), DriverID: uuid.New(), StaleAt: now.Add(-10 * time.Minute)}
	trip2 := positions.Task{TripID: uuid.New(), DriverID: uuid.New(), StaleAt: now.Add(-5 * time.Minute)}

	q.Push(trip2)
	q.Push(trip1)

	q.CloseInput()

	res, _ := q.Push(positions.Task{TripID: uuid.New(), DriverID: uuid.New(), StaleAt: now})
	if res != positions.ResultDropped {
		t.Fatalf("expected ResultDropped when pushing to closed queue, got %v", res)
	}

	ctx := context.Background()

	pop1, ok := q.Pop(ctx)
	if !ok || pop1.TripID != trip1.TripID {
		t.Fatalf("expected trip1, got %v", pop1.TripID)
	}

	pop2, ok := q.Pop(ctx)
	if !ok || pop2.TripID != trip2.TripID {
		t.Fatalf("expected trip2, got %v", pop2.TripID)
	}

	_, ok = q.Pop(ctx)
	if ok {
		t.Fatalf("expected Pop to return false after draining closed queue")
	}
}

func TestPriorityQueue_IdenticalTimestampsEviction(t *testing.T) {
	q := positions.NewPriorityQueue(2)
	now := time.Now().UTC()

	trip1 := positions.Task{TripID: uuid.New(), DriverID: uuid.New(), StaleAt: now}
	trip2 := positions.Task{TripID: uuid.New(), DriverID: uuid.New(), StaleAt: now}

	q.Push(trip1)
	q.Push(trip2)

	tripOlder := positions.Task{TripID: uuid.New(), DriverID: uuid.New(), StaleAt: now.Add(-5 * time.Minute)}

	res, evicted := q.Push(tripOlder)
	if res != positions.ResultEvicted {
		t.Fatalf("expected ResultEvicted, got %v", res)
	}
	if evicted == nil {
		t.Fatalf("expected non-nil evicted task")
	}

	ctx := context.Background()
	first, ok := q.Pop(ctx)
	if !ok || first.TripID != tripOlder.TripID {
		t.Fatalf("expected oldest task first, got %v", first.TripID)
	}
}

func TestPriorityQueue_PopContextCancelled(t *testing.T) {
	q := positions.NewPriorityQueue(5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, ok := q.Pop(ctx)
	if ok {
		t.Fatalf("expected false on cancelled context")
	}
}
