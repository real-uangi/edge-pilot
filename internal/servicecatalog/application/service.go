package application

import (
	"edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"strings"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
	commondb "github.com/real-uangi/allingo/common/db"
)

type Service struct {
	repo      domain.Repository
	publisher domain.ProxyConfigPublisher
}

func NewService(repo domain.Repository) *Service {
	return NewServiceWithPublisher(repo, nil)
}

func NewServiceWithPublisher(repo domain.Repository, publisher domain.ProxyConfigPublisher) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *Service) Create(req dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
	existing, err := s.repo.GetByKey(req.ServiceKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, business.NewBadRequest("serviceKey 已存在")
	}
	entity := buildServiceEntity(uuid.New(), req)
	if err := s.ensureRouteAvailable(entity.AgentID, entity.RouteHost, entity.RoutePathPrefix, entity.ID); err != nil {
		return nil, err
	}
	if err := s.repo.Create(entity); err != nil {
		return nil, err
	}
	if err := s.publishAgent(entity.AgentID); err != nil {
		return nil, err
	}
	output := toServiceOutput(entity)
	return &output, nil
}

func (s *Service) Update(id uuid.UUID, req dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
	current, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, business.ErrNotFound
	}
	updated := buildServiceEntity(id, req)
	updated.CreatedAt = current.CreatedAt
	updated.CurrentLiveSlot = current.CurrentLiveSlot
	if err := s.ensureRouteAvailable(updated.AgentID, updated.RouteHost, updated.RoutePathPrefix, updated.ID); err != nil {
		return nil, err
	}
	if err := s.repo.Update(updated); err != nil {
		return nil, err
	}
	if current.AgentID != updated.AgentID {
		if err := s.publishAgent(current.AgentID); err != nil {
			return nil, err
		}
	}
	if err := s.publishAgent(updated.AgentID); err != nil {
		return nil, err
	}
	output := toServiceOutput(updated)
	return &output, nil
}

func (s *Service) Get(id uuid.UUID) (*dto.ServiceOutput, error) {
	service, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, business.ErrNotFound
	}
	output := toServiceOutput(service)
	return &output, nil
}

func (s *Service) List() ([]dto.ServiceOutput, error) {
	services, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	output := make([]dto.ServiceOutput, 0, len(services))
	for i := range services {
		output = append(output, toServiceOutput(&services[i]))
	}
	return output, nil
}

func (s *Service) GetSpecByKey(key string) (*dto.ServiceDeploymentSpec, error) {
	service, err := s.repo.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, business.ErrNotFound
	}
	spec := toDeploymentSpec(service)
	return &spec, nil
}

func (s *Service) GetSpecByID(id uuid.UUID) (*dto.ServiceDeploymentSpec, error) {
	service, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, business.ErrNotFound
	}
	spec := toDeploymentSpec(service)
	return &spec, nil
}

func (s *Service) UpdateLiveSlot(id uuid.UUID, slot model.Slot) error {
	return s.repo.UpdateLiveSlot(id, slot)
}

func buildServiceEntity(id uuid.UUID, req dto.UpsertServiceRequest) *model.Service {
	dockerHealth := req.DockerHealthCheck
	if dockerHealth == nil {
		dockerHealth = boolPointer(true)
	}
	enabled := req.Enabled
	if enabled == nil {
		enabled = boolPointer(true)
	}
	expectedCode := req.HTTPExpectedCode
	if expectedCode == 0 {
		expectedCode = 200
	}
	timeoutSeconds := req.HTTPTimeoutSecond
	if timeoutSeconds == 0 {
		timeoutSeconds = 5
	}

	return &model.Service{
		ID:                id,
		ServiceKey:        req.ServiceKey,
		Name:              req.Name,
		AgentID:           req.AgentID,
		ImageRepo:         req.ImageRepo,
		ContainerPort:     req.ContainerPort,
		BlueHostPort:      req.BlueHostPort,
		GreenHostPort:     req.GreenHostPort,
		DockerHealthCheck: dockerHealth,
		HTTPHealthPath:    req.HTTPHealthPath,
		HTTPExpectedCode:  expectedCode,
		HTTPTimeoutSecond: timeoutSeconds,
		RouteHost:         NormalizeRouteHost(req.RouteHost),
		RoutePathPrefix:   NormalizeRoutePathPrefix(req.RoutePathPrefix),
		Env:               commondb.NewJSONB(req.Env),
		Command:           commondb.NewJSONB(req.Command),
		Entrypoint:        commondb.NewJSONB(req.Entrypoint),
		Volumes:           commondb.NewJSONB(toModelVolumes(req.Volumes)),
		Enabled:           enabled,
	}
}

func toServiceOutput(service *model.Service) dto.ServiceOutput {
	return dto.ServiceOutput{
		ID:                service.ID,
		Name:              service.Name,
		ServiceKey:        service.ServiceKey,
		AgentID:           service.AgentID,
		ImageRepo:         service.ImageRepo,
		ContainerPort:     service.ContainerPort,
		BlueHostPort:      service.BlueHostPort,
		GreenHostPort:     service.GreenHostPort,
		CurrentLiveSlot:   service.CurrentLiveSlot,
		DockerHealthCheck: service.DockerHealthCheck,
		HTTPHealthPath:    service.HTTPHealthPath,
		HTTPExpectedCode:  service.HTTPExpectedCode,
		HTTPTimeoutSecond: service.HTTPTimeoutSecond,
		RouteHost:         service.RouteHost,
		RoutePathPrefix:   service.RoutePathPrefix,
		Env:               getJSON(service.Env),
		Command:           getJSON(service.Command),
		Entrypoint:        getJSON(service.Entrypoint),
		Volumes:           toDTOVolumes(getJSON(service.Volumes)),
		Enabled:           service.Enabled,
		CreatedAt:         service.CreatedAt,
		UpdatedAt:         service.UpdatedAt,
	}
}

func toDeploymentSpec(service *model.Service) dto.ServiceDeploymentSpec {
	return dto.ServiceDeploymentSpec{
		ID:                service.ID,
		Name:              service.Name,
		ServiceKey:        service.ServiceKey,
		AgentID:           service.AgentID,
		ImageRepo:         service.ImageRepo,
		ContainerPort:     service.ContainerPort,
		BlueHostPort:      service.BlueHostPort,
		GreenHostPort:     service.GreenHostPort,
		CurrentLiveSlot:   service.CurrentLiveSlot,
		DockerHealthCheck: service.DockerHealthCheck != nil && *service.DockerHealthCheck,
		HTTPHealthPath:    service.HTTPHealthPath,
		HTTPExpectedCode:  service.HTTPExpectedCode,
		HTTPTimeoutSecond: service.HTTPTimeoutSecond,
		RouteHost:         service.RouteHost,
		RoutePathPrefix:   service.RoutePathPrefix,
		Env:               getJSON(service.Env),
		Command:           getJSON(service.Command),
		Entrypoint:        getJSON(service.Entrypoint),
		Volumes:           toDTOVolumes(getJSON(service.Volumes)),
		Enabled:           service.Enabled != nil && *service.Enabled,
	}
}

func toModelVolumes(items []dto.VolumeMount) []model.VolumeMount {
	out := make([]model.VolumeMount, 0, len(items))
	for _, item := range items {
		out = append(out, model.VolumeMount{
			Source:   item.Source,
			Target:   item.Target,
			ReadOnly: item.ReadOnly,
		})
	}
	return out
}

func toDTOVolumes(items []model.VolumeMount) []dto.VolumeMount {
	out := make([]dto.VolumeMount, 0, len(items))
	for _, item := range items {
		out = append(out, dto.VolumeMount{
			Source:   item.Source,
			Target:   item.Target,
			ReadOnly: item.ReadOnly,
		})
	}
	return out
}

func boolPointer(v bool) *bool {
	return &v
}

func (s *Service) ensureRouteAvailable(agentID string, routeHost string, routePathPrefix string, selfID uuid.UUID) error {
	existing, err := s.repo.GetByRoute(agentID, routeHost, routePathPrefix)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != selfID {
		return business.NewBadRequest("routeHost + routePathPrefix 已存在")
	}
	return nil
}

func (s *Service) publishAgent(agentID string) error {
	if s.publisher == nil || strings.TrimSpace(agentID) == "" {
		return nil
	}
	return s.publisher.PublishAgent(agentID)
}

func getJSON[T any](value *commondb.JSONB[T]) T {
	var zero T
	if value == nil {
		return zero
	}
	return value.Get()
}
