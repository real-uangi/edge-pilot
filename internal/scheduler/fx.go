package scheduler

import (
	"github.com/real-uangi/edge-pilot/internal/scheduler/application"
	"github.com/real-uangi/edge-pilot/internal/scheduler/domain"
	"github.com/real-uangi/edge-pilot/internal/scheduler/infra"
	servicecatalogapp "github.com/real-uangi/edge-pilot/internal/servicecatalog/application"
	"github.com/real-uangi/edge-pilot/internal/shared/model"

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
		infra.NewRepository,
		provideLiveSlotResolver,
		application.NewService,
	),
)
