package application

import (
	"sync"
	"testing"
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/perf"
)

func TestSnapshotRingWrapAround(t *testing.T) {
	ring := newSnapshotRing(3)
	base := time.Now()
	ring.append(perf.Snapshot{CPUPercent: 1, CollectedAt: base})
	ring.append(perf.Snapshot{CPUPercent: 2, CollectedAt: base.Add(time.Second)})
	ring.append(perf.Snapshot{CPUPercent: 3, CollectedAt: base.Add(2 * time.Second)})
	ring.append(perf.Snapshot{CPUPercent: 4, CollectedAt: base.Add(3 * time.Second)})

	points := ring.list()
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	if points[0].CPUPercent != 2 || points[1].CPUPercent != 3 || points[2].CPUPercent != 4 {
		t.Fatalf("unexpected ring order: %#v", points)
	}
	latest, ok := ring.latest()
	if !ok {
		t.Fatal("expected latest point")
	}
	if latest.CPUPercent != 4 {
		t.Fatalf("expected latest cpu=4, got %v", latest.CPUPercent)
	}
}

func TestSnapshotRingConcurrentReadWrite(t *testing.T) {
	ring := newSnapshotRing(10)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(value int) {
			defer wg.Done()
			ring.append(perf.Snapshot{CPUPercent: float64(value)})
			_, _ = ring.latest()
			_ = ring.list()
		}(i)
	}
	wg.Wait()
	points := ring.list()
	if len(points) == 0 {
		t.Fatal("expected points after concurrent writes")
	}
	if len(points) > 10 {
		t.Fatalf("ring overflowed capacity: %d", len(points))
	}
}
