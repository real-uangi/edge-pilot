package config

import (
	"os"
	"strings"
	"time"
)

type SchedulerConfig struct {
	EngineTickInterval time.Duration
	DispatchBatchSize  int
	DefaultLeaseSec    int
	DefaultMaxRetries  int
	HeartbeatTimeout   time.Duration
}

func LoadSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		EngineTickInterval: time.Duration(defaultInt(os.Getenv("SCHEDULER_ENGINE_TICK_SECONDS"), 2)) * time.Second,
		DispatchBatchSize:  defaultInt(os.Getenv("SCHEDULER_DISPATCH_BATCH_SIZE"), 100),
		DefaultLeaseSec:    defaultInt(os.Getenv("SCHEDULER_DEFAULT_LEASE_TIMEOUT_SECONDS"), 60),
		DefaultMaxRetries:  defaultInt(os.Getenv("SCHEDULER_DEFAULT_MAX_RETRIES"), 3),
		HeartbeatTimeout:   time.Duration(defaultInt(os.Getenv("SCHEDULER_EXECUTOR_HEARTBEAT_TIMEOUT_SECONDS"), 15)) * time.Second,
	}
}

func ParseSchedulerScheduleKind(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "one_time"
	}
	return value
}
