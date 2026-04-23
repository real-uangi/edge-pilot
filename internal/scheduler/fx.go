package scheduler

import (
	"edge-pilot/internal/scheduler/application"
	"edge-pilot/internal/scheduler/domain"
	"edge-pilot/internal/scheduler/infra"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/model"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

type serviceCatalogLiveSlotResolver struct {
	services *servicecatalogapp.Service
}

func (r *serviceCatalogLiveSlotResolver) ResolveLiveSlot(serviceID uuid.UUID) (model.Slot, error) {
	spec, err := r.services.GetSpecByID(serviceID)
	if err != nil {
		return 0, err
	}
	return spec.CurrentLiveSlot, nil
}

func provideLiveSlotResolver(services *servicecatalogapp.Service) domain.LiveSlotResolver {
	return &serviceCatalogLiveSlotResolver{services: services}
}

var ControlPlaneModule = fx.Module(
	"scheduler",
	fx.Provide(
		config.LoadSchedulerConfig,
		config.LoadAgentAuthConfig,
		infra.NewRepository,
		provideLiveSlotResolver,
		application.NewService,
	),
)
