package perf

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectFromCgroupV2(t *testing.T) {
	root := t.TempDir()
	procPath := filepath.Join(root, "proc.self.cgroup")
	cgroupPath := filepath.Join(root, "test-group")
	if err := os.MkdirAll(cgroupPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(procPath, []byte("0::/test-group\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(proc) error = %v", err)
	}
	writeCgroupV2File(t, cgroupPath, "cpu.stat", "usage_usec 100000\n")
	writeCgroupV2File(t, cgroupPath, "cpu.max", "50000 100000\n")
	writeCgroupV2File(t, cgroupPath, "memory.current", "1024\n")
	writeCgroupV2File(t, cgroupPath, "memory.max", "2048\n")

	now := time.Unix(1700000000, 0)
	c := NewCollector(
		WithProcCgroupPath(procPath),
		WithCgroupRoot(root),
		withNow(func() time.Time { return now }),
		withFallbackProvider(&fakeFallbackProvider{}),
	).(*collector)
	first, err := c.collectFromCgroup(now)
	if err != nil {
		t.Fatalf("collectFromCgroup(first) error = %v", err)
	}
	if first.CPUPercent != 0 {
		t.Fatalf("expected first sample cpu=0, got %v", first.CPUPercent)
	}
	writeCgroupV2File(t, cgroupPath, "cpu.stat", "usage_usec 200000\n")

	secondTime := now.Add(time.Second)
	second, err := c.collectFromCgroup(secondTime)
	if err != nil {
		t.Fatalf("collectFromCgroup(second) error = %v", err)
	}
	if second.Source != SourceCgroup {
		t.Fatalf("expected cgroup source, got %s", second.Source)
	}
	if second.MemoryUsedBytes != 1024 || second.MemoryLimitBytes != 2048 {
		t.Fatalf("unexpected memory values: %#v", second)
	}
	if second.CPUPercent < 19.9 || second.CPUPercent > 20.1 {
		t.Fatalf("expected cpu around 20%%, got %v", second.CPUPercent)
	}
}

func TestCollectFallsBackWhenCgroupUnavailable(t *testing.T) {
	c := NewCollector(
		WithProcCgroupPath(filepath.Join(t.TempDir(), "missing")),
		withFallbackProvider(&fakeFallbackProvider{
			cpuPercent: 60,
			rss:        4096,
			total:      8192,
		}),
		withNow(func() time.Time { return time.Unix(1700000000, 0) }),
	).(*collector)
	snapshot, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if snapshot.Source != SourceGopsutil {
		t.Fatalf("expected gopsutil source, got %s", snapshot.Source)
	}
	if snapshot.MemoryUsedBytes != 4096 || snapshot.MemoryLimitBytes != 8192 {
		t.Fatalf("unexpected memory values: %#v", snapshot)
	}
}

func TestCollectFromCgroupV1(t *testing.T) {
	root := t.TempDir()
	procPath := filepath.Join(root, "proc.self.cgroup")
	cpuPath := filepath.Join(root, "cpu", "test")
	memPath := filepath.Join(root, "memory", "test")
	if err := os.MkdirAll(cpuPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(cpu) error = %v", err)
	}
	if err := os.MkdirAll(memPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(memory) error = %v", err)
	}
	if err := os.WriteFile(procPath, []byte("2:cpu,cpuacct:/test\n1:memory:/test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(proc) error = %v", err)
	}
	writeCgroupV2File(t, cpuPath, "cpuacct.usage", "100000000\n")
	writeCgroupV2File(t, cpuPath, "cpu.cfs_quota_us", "100000\n")
	writeCgroupV2File(t, cpuPath, "cpu.cfs_period_us", "100000\n")
	writeCgroupV2File(t, memPath, "memory.usage_in_bytes", "2048\n")
	writeCgroupV2File(t, memPath, "memory.limit_in_bytes", "4096\n")

	now := time.Unix(1700000000, 0)
	c := NewCollector(
		WithProcCgroupPath(procPath),
		WithCgroupRoot(root),
		withNow(func() time.Time { return now }),
		withFallbackProvider(&fakeFallbackProvider{}),
	).(*collector)
	_, err := c.collectFromCgroup(now)
	if err != nil {
		t.Fatalf("collectFromCgroup(first) error = %v", err)
	}
	writeCgroupV2File(t, cpuPath, "cpuacct.usage", "200000000\n")
	second, err := c.collectFromCgroup(now.Add(time.Second))
	if err != nil {
		t.Fatalf("collectFromCgroup(second) error = %v", err)
	}
	if second.Source != SourceCgroup {
		t.Fatalf("expected cgroup source, got %s", second.Source)
	}
	if second.CPUPercent < 9.9 || second.CPUPercent > 10.1 {
		t.Fatalf("expected cpu around 10%%, got %v", second.CPUPercent)
	}
}

func writeCgroupV2File(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}

type fakeFallbackProvider struct {
	cpuPercent float64
	rss        uint64
	total      uint64
	err        error
}

func (f *fakeFallbackProvider) CPUPercent() (float64, error) {
	return f.cpuPercent, f.err
}

func (f *fakeFallbackProvider) MemoryRSS() (uint64, error) {
	return f.rss, f.err
}

func (f *fakeFallbackProvider) MemoryTotal() (uint64, error) {
	return f.total, f.err
}
