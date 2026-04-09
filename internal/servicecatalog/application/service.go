package application

import (
	"edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
	commondb "github.com/real-uangi/allingo/common/db"
)

type Service struct {
	repo      domain.Repository
	publisher domain.ProxyConfigPublisher
	agents    domain.AgentLookup
}

func NewService(repo domain.Repository) *Service {
	return NewServiceWithPublisher(repo, nil, nil)
}

func NewServiceWithPublisher(repo domain.Repository, publisher domain.ProxyConfigPublisher, agents domain.AgentLookup) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
		agents:    agents,
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
	if err := validatePublishedPorts(entity.PublishedPorts.Get()); err != nil {
		return nil, err
	}
	if err := s.ensureAgentAssignable(entity.AgentID); err != nil {
		return nil, err
	}
	if err := s.ensureRouteAvailable(entity.AgentID, entity.RouteHost, entity.RoutePathPrefix, entity.ID); err != nil {
		return nil, err
	}
	if err := s.ensurePublishedPortsAvailable(entity.AgentID, entity.PublishedPorts.Get(), entity.ID); err != nil {
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
	if err := validatePublishedPorts(updated.PublishedPorts.Get()); err != nil {
		return nil, err
	}
	if err := s.ensureAgentAssignable(updated.AgentID); err != nil {
		return nil, err
	}
	if err := s.ensureRouteAvailable(updated.AgentID, updated.RouteHost, updated.RoutePathPrefix, updated.ID); err != nil {
		return nil, err
	}
	if err := s.ensurePublishedPortsAvailable(updated.AgentID, updated.PublishedPorts.Get(), updated.ID); err != nil {
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
		PublishedPorts:    commondb.NewJSONB(toModelPublishedPorts(req.PublishedPorts)),
		Enabled:           enabled,
	}
}

func (s *Service) ensureAgentAssignable(agentID string) error {
	if _, err := uuid.Parse(agentID); err != nil {
		return business.NewBadRequest("agentId 必须是 UUID")
	}
	if s.agents == nil {
		return nil
	}
	agent, err := s.agents.GetAgent(agentID)
	if err != nil {
		if err == business.ErrNotFound {
			return business.NewBadRequest("agentId 不存在或已禁用")
		}
		return err
	}
	if agent == nil {
		return business.NewBadRequest("agentId 不存在或已禁用")
	}
	if agent.Enabled == nil || !*agent.Enabled {
		return business.NewBadRequest("agentId 不存在或已禁用")
	}
	return nil
}

func toServiceOutput(service *model.Service) dto.ServiceOutput {
	return dto.ServiceOutput{
		ID:                service.ID,
		Name:              service.Name,
		ServiceKey:        service.ServiceKey,
		AgentID:           service.AgentID,
		ImageRepo:         service.ImageRepo,
		ContainerPort:     service.ContainerPort,
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
		PublishedPorts:    toDTOPublishedPorts(getJSON(service.PublishedPorts)),
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
		PublishedPorts:    toDTOPublishedPorts(getJSON(service.PublishedPorts)),
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

func toModelPublishedPorts(items []dto.PublishedPort) []model.PublishedPort {
	out := make([]model.PublishedPort, 0, len(items))
	for _, item := range items {
		out = append(out, model.PublishedPort{
			HostPort:      item.HostPort,
			ContainerPort: item.ContainerPort,
		})
	}
	return out
}

func toDTOPublishedPorts(items []model.PublishedPort) []dto.PublishedPort {
	out := make([]dto.PublishedPort, 0, len(items))
	for _, item := range items {
		out = append(out, dto.PublishedPort{
			HostPort:      item.HostPort,
			ContainerPort: item.ContainerPort,
		})
	}
	return out
}

func boolPointer(v bool) *bool {
	return &v
}

func validatePublishedPorts(items []model.PublishedPort) error {
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		if item.HostPort <= 0 || item.HostPort > 65535 {
			return business.NewBadRequest("publishedPorts.hostPort 非法")
		}
		if item.ContainerPort <= 0 || item.ContainerPort > 65535 {
			return business.NewBadRequest("publishedPorts.containerPort 非法")
		}
		if item.HostPort == SharedFrontendBindPort {
			return business.NewBadRequest("publishedPorts.hostPort 与代理保留端口冲突")
		}
		if _, ok := seen[item.HostPort]; ok {
			return business.NewBadRequest("publishedPorts.hostPort 重复")
		}
		seen[item.HostPort] = struct{}{}
	}
	return nil
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

func (s *Service) ensurePublishedPortsAvailable(agentID string, ports []model.PublishedPort, selfID uuid.UUID) error {
	if len(ports) == 0 {
		return nil
	}
	services, err := s.repo.ListByAgent(agentID)
	if err != nil {
		return err
	}
	for _, candidate := range ports {
		for i := range services {
			if services[i].ID == selfID {
				continue
			}
			for _, port := range getJSON(services[i].PublishedPorts) {
				if port.HostPort == candidate.HostPort {
					return business.NewBadRequest(fmt.Sprintf("publishedPorts.hostPort 已被服务 %s 占用", services[i].ServiceKey))
				}
			}
		}
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
