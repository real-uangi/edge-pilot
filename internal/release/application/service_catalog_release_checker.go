package application

import (
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogdomain "edge-pilot/internal/servicecatalog/domain"

	"github.com/google/uuid"
)

type ServiceCatalogReleaseChecker struct {
	repo releasedomain.Repository
}

func NewServiceCatalogReleaseChecker(repo releasedomain.Repository) servicecatalogdomain.ReleaseStateChecker {
	return &ServiceCatalogReleaseChecker{repo: repo}
}

func (c *ServiceCatalogReleaseChecker) HasActiveRelease(serviceID uuid.UUID) (bool, error) {
	return c.repo.HasActiveRelease(serviceID)
}

func (c *ServiceCatalogReleaseChecker) HasTrafficSplitRelease(serviceID uuid.UUID) (bool, error) {
	return c.repo.HasTrafficSplitRelease(serviceID)
}
