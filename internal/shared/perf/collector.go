package perf

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	gopsutilmem "github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	SourceCgroup   = "cgroup"
	SourceGopsutil = "gopsutil"
)

const unlimitedMemoryThreshold = 1 << 60

type Snapshot struct {
	CPUPercent       float64
	MemoryUsedBytes  uint64
	MemoryLimitBytes uint64
	Source           string
	CollectedAt      time.Time
}

type Collector interface {
	Collect(context.Context) (*Snapshot, error)
}

type Option func(*collector)

type fallbackProvider interface {
	CPUPercent() (float64, error)
	MemoryRSS() (uint64, error)
	MemoryTotal() (uint64, error)
}

type collector struct {
	procCgroupPath string
	cgroupRoot     string
	now            func() time.Time
	hostCPUCores   float64
	fallback       fallbackProvider

	mu          sync.Mutex
	lastUsageNs uint64
	lastAt      time.Time
}

func NewCollector(options ...Option) Collector {
	c := &collector{
		procCgroupPath: "/proc/self/cgroup",
		cgroupRoot:     "/sys/fs/cgroup",
		now:            time.Now,
		hostCPUCores:   float64(runtime.NumCPU()),
		fallback:       newGopsutilFallbackProvider(),
	}
	for _, option := range options {
		option(c)
	}
	if c.hostCPUCores <= 0 {
		c.hostCPUCores = 1
	}
	return c
}

func WithProcCgroupPath(path string) Option {
	return func(c *collector) {
		if strings.TrimSpace(path) != "" {
			c.procCgroupPath = path
		}
	}
}

func WithCgroupRoot(path string) Option {
	return func(c *collector) {
		if strings.TrimSpace(path) != "" {
			c.cgroupRoot = path
		}
	}
}

func withNow(now func() time.Time) Option {
	return func(c *collector) {
		if now != nil {
			c.now = now
		}
	}
}

func withFallbackProvider(provider fallbackProvider) Option {
	return func(c *collector) {
		if provider != nil {
			c.fallback = provider
		}
	}
}

func (c *collector) Collect(_ context.Context) (*Snapshot, error) {
	collectedAt := c.now()
	snapshot, err := c.collectFromCgroup(collectedAt)
	if err == nil {
		return snapshot, nil
	}
	fallbackSnapshot, fallbackErr := c.collectFromFallback(collectedAt)
	if fallbackErr != nil {
		return nil, errors.Join(err, fallbackErr)
	}
	return fallbackSnapshot, nil
}

func (c *collector) collectFromFallback(collectedAt time.Time) (*Snapshot, error) {
	if c.fallback == nil {
		return nil, errors.New("fallback provider unavailable")
	}
	cpuPercent, err := c.fallback.CPUPercent()
	if err != nil {
		return nil, err
	}
	memoryRSS, err := c.fallback.MemoryRSS()
	if err != nil {
		return nil, err
	}
	memoryTotal, err := c.fallback.MemoryTotal()
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		CPUPercent:       normalizePercent(cpuPercent, c.hostCPUCores),
		MemoryUsedBytes:  memoryRSS,
		MemoryLimitBytes: memoryTotal,
		Source:           SourceGopsutil,
		CollectedAt:      collectedAt,
	}, nil
}

func (c *collector) collectFromCgroup(collectedAt time.Time) (*Snapshot, error) {
	raw, err := os.ReadFile(c.procCgroupPath)
	if err != nil {
		return nil, err
	}
	info := parseProcCgroup(string(raw))
	if info.v2Path != "" {
		return c.collectFromV2(info.v2Path, collectedAt)
	}
	return c.collectFromV1(info.cpuPath, info.memoryPath, collectedAt)
}

func (c *collector) collectFromV2(cgroupPath string, collectedAt time.Time) (*Snapshot, error) {
	basePath := filepath.Join(c.cgroupRoot, cleanCgroupPath(cgroupPath))
	cpuStatRaw, err := os.ReadFile(filepath.Join(basePath, "cpu.stat"))
	if err != nil {
		return nil, err
	}
	usageNs, err := parseV2CPUUsage(cpuStatRaw)
	if err != nil {
		return nil, err
	}
	cpuMaxRaw, err := os.ReadFile(filepath.Join(basePath, "cpu.max"))
	if err != nil {
		return nil, err
	}
	cpuLimitCores, err := parseV2CPULimit(cpuMaxRaw)
	if err != nil {
		return nil, err
	}
	memoryCurrentRaw, err := os.ReadFile(filepath.Join(basePath, "memory.current"))
	if err != nil {
		return nil, err
	}
	memoryUsed, err := parseUint(memoryCurrentRaw)
	if err != nil {
		return nil, err
	}
	memoryMaxRaw, err := os.ReadFile(filepath.Join(basePath, "memory.max"))
	if err != nil {
		return nil, err
	}
	memoryLimit, err := parseV2MemoryLimit(memoryMaxRaw)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		CPUPercent:       c.calculateCPUPercent(usageNs, collectedAt, cpuLimitCores),
		MemoryUsedBytes:  memoryUsed,
		MemoryLimitBytes: memoryLimit,
		Source:           SourceCgroup,
		CollectedAt:      collectedAt,
	}, nil
}

func (c *collector) collectFromV1(cpuCgroupPath string, memoryCgroupPath string, collectedAt time.Time) (*Snapshot, error) {
	cpuBasePath, err := c.resolveV1CPUBasePath(cpuCgroupPath)
	if err != nil {
		return nil, err
	}
	memoryBasePath, err := c.resolveV1MemoryBasePath(memoryCgroupPath)
	if err != nil {
		return nil, err
	}
	usageRaw, err := os.ReadFile(filepath.Join(cpuBasePath, "cpuacct.usage"))
	if err != nil {
		return nil, err
	}
	usageNs, err := parseUint(usageRaw)
	if err != nil {
		return nil, err
	}
	quotaRaw, err := os.ReadFile(filepath.Join(cpuBasePath, "cpu.cfs_quota_us"))
	if err != nil {
		return nil, err
	}
	periodRaw, err := os.ReadFile(filepath.Join(cpuBasePath, "cpu.cfs_period_us"))
	if err != nil {
		return nil, err
	}
	cpuLimitCores, err := parseV1CPULimit(quotaRaw, periodRaw)
	if err != nil {
		return nil, err
	}
	memoryUsageRaw, err := os.ReadFile(filepath.Join(memoryBasePath, "memory.usage_in_bytes"))
	if err != nil {
		return nil, err
	}
	memoryUsed, err := parseUint(memoryUsageRaw)
	if err != nil {
		return nil, err
	}
	memoryLimitRaw, err := os.ReadFile(filepath.Join(memoryBasePath, "memory.limit_in_bytes"))
	if err != nil {
		return nil, err
	}
	memoryLimit, err := parseV1MemoryLimit(memoryLimitRaw)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		CPUPercent:       c.calculateCPUPercent(usageNs, collectedAt, cpuLimitCores),
		MemoryUsedBytes:  memoryUsed,
		MemoryLimitBytes: memoryLimit,
		Source:           SourceCgroup,
		CollectedAt:      collectedAt,
	}, nil
}

func (c *collector) resolveV1CPUBasePath(cgroupPath string) (string, error) {
	relativePath := cleanCgroupPath(cgroupPath)
	candidates := []string{
		filepath.Join(c.cgroupRoot, "cpu", relativePath),
		filepath.Join(c.cgroupRoot, "cpuacct", relativePath),
		filepath.Join(c.cgroupRoot, "cpu,cpuacct", relativePath),
		filepath.Join(c.cgroupRoot, relativePath),
	}
	return findPathWithFile(candidates, "cpuacct.usage")
}

func (c *collector) resolveV1MemoryBasePath(cgroupPath string) (string, error) {
	relativePath := cleanCgroupPath(cgroupPath)
	candidates := []string{
		filepath.Join(c.cgroupRoot, "memory", relativePath),
		filepath.Join(c.cgroupRoot, relativePath),
	}
	return findPathWithFile(candidates, "memory.usage_in_bytes")
}

func findPathWithFile(candidates []string, fileName string) (string, error) {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		_, err := os.Stat(filepath.Join(candidate, fileName))
		if err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cgroup file %s not found", fileName)
}

func (c *collector) calculateCPUPercent(currentUsageNs uint64, collectedAt time.Time, limitCores float64) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastAt.IsZero() || currentUsageNs < c.lastUsageNs {
		c.lastUsageNs = currentUsageNs
		c.lastAt = collectedAt
		return 0
	}
	elapsedNs := collectedAt.Sub(c.lastAt).Nanoseconds()
	usageDeltaNs := currentUsageNs - c.lastUsageNs
	c.lastUsageNs = currentUsageNs
	c.lastAt = collectedAt
	if elapsedNs <= 0 {
		return 0
	}
	cpuCores := limitCores
	if cpuCores <= 0 {
		cpuCores = c.hostCPUCores
	}
	if cpuCores <= 0 {
		cpuCores = 1
	}
	rawPercent := (float64(usageDeltaNs) / float64(elapsedNs)) * 100 / cpuCores
	return normalizePercent(rawPercent, 1)
}

func normalizePercent(percent float64, divisor float64) float64 {
	if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 {
		return 0
	}
	if divisor > 0 {
		percent /= divisor
	}
	if percent > 100 {
		return 100
	}
	return percent
}

type procCgroupInfo struct {
	v2Path     string
	cpuPath    string
	memoryPath string
}

func parseProcCgroup(raw string) procCgroupInfo {
	info := procCgroupInfo{}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		controllers := parts[1]
		path := parts[2]
		if controllers == "" {
			info.v2Path = path
			continue
		}
		controllerList := strings.Split(controllers, ",")
		for _, controller := range controllerList {
			switch strings.TrimSpace(controller) {
			case "cpu", "cpuacct":
				if info.cpuPath == "" {
					info.cpuPath = path
				}
			case "memory":
				if info.memoryPath == "" {
					info.memoryPath = path
				}
			}
		}
	}
	return info
}

func cleanCgroupPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)
	if path == "." {
		return ""
	}
	return path
}

func parseV2CPUUsage(raw []byte) (uint64, error) {
	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "usage_usec":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return value * uint64(time.Microsecond), nil
		case "usage_nsec":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return value, nil
		}
	}
	return 0, errors.New("usage not found in cpu.stat")
}

func parseV2CPULimit(raw []byte) (float64, error) {
	fields := strings.Fields(strings.TrimSpace(string(raw)))
	if len(fields) != 2 {
		return 0, errors.New("invalid cpu.max format")
	}
	if fields[0] == "max" {
		return 0, nil
	}
	quota, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	period, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, err
	}
	if quota <= 0 || period <= 0 {
		return 0, nil
	}
	return quota / period, nil
}

func parseV2MemoryLimit(raw []byte) (uint64, error) {
	text := strings.TrimSpace(string(raw))
	if text == "max" || text == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, err
	}
	if value >= unlimitedMemoryThreshold {
		return 0, nil
	}
	return value, nil
}

func parseV1CPULimit(quotaRaw []byte, periodRaw []byte) (float64, error) {
	quotaText := strings.TrimSpace(string(quotaRaw))
	periodText := strings.TrimSpace(string(periodRaw))
	if quotaText == "" || periodText == "" {
		return 0, errors.New("empty cfs quota/period")
	}
	quota, err := strconv.ParseFloat(quotaText, 64)
	if err != nil {
		return 0, err
	}
	period, err := strconv.ParseFloat(periodText, 64)
	if err != nil {
		return 0, err
	}
	if quota <= 0 || period <= 0 {
		return 0, nil
	}
	return quota / period, nil
}

func parseV1MemoryLimit(raw []byte) (uint64, error) {
	value, err := parseUint(raw)
	if err != nil {
		return 0, err
	}
	if value >= unlimitedMemoryThreshold {
		return 0, nil
	}
	return value, nil
}

func parseUint(raw []byte) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
}

type gopsutilFallbackProvider struct {
	process *process.Process
}

func newGopsutilFallbackProvider() fallbackProvider {
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return nil
	}
	return &gopsutilFallbackProvider{process: proc}
}

func (p *gopsutilFallbackProvider) CPUPercent() (float64, error) {
	return p.process.Percent(0)
}

func (p *gopsutilFallbackProvider) MemoryRSS() (uint64, error) {
	info, err := p.process.MemoryInfo()
	if err != nil {
		return 0, err
	}
	return info.RSS, nil
}

func (p *gopsutilFallbackProvider) MemoryTotal() (uint64, error) {
	info, err := gopsutilmem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return info.Total, nil
}
