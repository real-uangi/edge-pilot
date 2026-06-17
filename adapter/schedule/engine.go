package schedule

import (
	"context"
	"time"

	schedulerapp "github.com/real-uangi/edge-pilot/internal/scheduler/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

type SchedulerEngine struct {
	service    *schedulerapp.Service
	dispatcher schedulerapp.RunDispatcher
	cfg        *config.SchedulerConfig
	logger     *log.StdLogger
}

type schedulerEngineDeps struct {
	fx.In

	Service    *schedulerapp.Service
	Config     *config.SchedulerConfig
	Dispatcher schedulerapp.RunDispatcher `optional:"true"`
}

func NewSchedulerEngine(deps schedulerEngineDeps) *SchedulerEngine {
	return &SchedulerEngine{
		service:    deps.Service,
		dispatcher: deps.Dispatcher,
		cfg:        deps.Config,
		logger:     log.NewStdLogger("schedule.engine"),
	}
}

func startSchedulerEngine(lc fx.Lifecycle, engine *SchedulerEngine) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go engine.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func (s *SchedulerEngine) run(ctx context.Context) {
	tick := s.cfg.EngineTickInterval
	if tick <= 0 {
		tick = 2 * time.Second
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC()
			if err := s.service.EnqueueDueJobs(now); err != nil {
				s.logger.Errorf(err, "enqueue due scheduler jobs failed")
			}
			if s.dispatcher != nil {
				if err := s.service.DispatchDueRuns(now, s.dispatcher); err != nil {
					s.logger.Errorf(err, "dispatch scheduler runs failed")
				}
			}
		}
	}
}
