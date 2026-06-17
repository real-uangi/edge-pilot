package application

import (
	"sync"

	"github.com/real-uangi/edge-pilot/internal/shared/perf"
)

type snapshotRing struct {
	mu       sync.RWMutex
	capacity int
	items    []perf.Snapshot
	head     int
	size     int
}

func newSnapshotRing(capacity int) *snapshotRing {
	if capacity <= 0 {
		capacity = 1
	}
	return &snapshotRing{
		capacity: capacity,
		items:    make([]perf.Snapshot, capacity),
	}
}

func (r *snapshotRing) append(item perf.Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[r.head] = item
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}
}

func (r *snapshotRing) latest() (perf.Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.size == 0 {
		return perf.Snapshot{}, false
	}
	index := r.head - 1
	if index < 0 {
		index = r.capacity - 1
	}
	return r.items[index], true
}

func (r *snapshotRing) list() []perf.Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.size == 0 {
		return nil
	}
	out := make([]perf.Snapshot, 0, r.size)
	start := r.head - r.size
	if start < 0 {
		start += r.capacity
	}
	for i := 0; i < r.size; i++ {
		idx := (start + i) % r.capacity
		out = append(out, r.items[idx])
	}
	return out
}
