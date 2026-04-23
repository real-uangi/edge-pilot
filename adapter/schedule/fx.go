package schedule

import "go.uber.org/fx"

var Module = fx.Module(
	"schedule",
	fx.Provide(NewRecoveryScheduler),
	fx.Provide(NewSchedulerEngine),
	fx.Invoke(
		startRecoveryScheduler,
		startSchedulerEngine,
	),
)
