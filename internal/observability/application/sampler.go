package application

import (
	"context"
	"time"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

const PerformanceSampleInterval = 15 * time.Second

func StartControlPlanePerformanceSampler(lc fx.Lifecycle, service *Service) {
	logger := log.NewStdLogger("observability.sampler")
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if err := service.SampleControlPlanePerformance(ctx); err != nil {
				logger.Errorf(err, "initial control-plane performance sampling failed")
			}
			ticker := time.NewTicker(PerformanceSampleInterval)
			go func() {
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						if err := service.SampleControlPlanePerformance(ctx); err != nil {
							logger.Errorf(err, "control-plane performance sampling failed")
						}
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}
