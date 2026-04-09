package schedule

import (
	"context"
	agentapp "edge-pilot/internal/agent/application"
	releaseapp "edge-pilot/internal/release/application"
	"time"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

const (
	offlineAfter = 15 * time.Second
	taskTimeout  = 10 * time.Minute
	scanInterval = 5 * time.Second
)

type RecoveryScheduler struct {
	agents   *agentapp.RegistryService
	releases *releaseapp.Service
	logger   *log.StdLogger
}

func NewRecoveryScheduler(agents *agentapp.RegistryService, releases *releaseapp.Service) *RecoveryScheduler {
	return &RecoveryScheduler{
		agents:   agents,
		releases: releases,
		logger:   log.NewStdLogger("schedule.recovery"),
	}
}

func startRecoveryScheduler(lc fx.Lifecycle, scheduler *RecoveryScheduler) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go scheduler.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func (s *RecoveryScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.tick(); err != nil {
				s.logger.Errorf(err, "recovery scheduler tick failed")
			}
		}
	}
}

func (s *RecoveryScheduler) tick() error {
	staleAgents, err := s.agents.MarkOfflineStale(time.Now().Add(-offlineAfter))
	if err != nil {
		return err
	}
	for _, agentID := range staleAgents {
		if err := s.releases.RecordAgentOfflineTimeout(agentID); err != nil {
			return err
		}
	}
	return s.releases.FailStaleTasks(time.Now().Add(-taskTimeout))
}
