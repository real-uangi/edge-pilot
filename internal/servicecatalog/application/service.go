package application

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/real-uangi/edge-pilot/internal/servicecatalog/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/model"
	"github.com/real-uangi/edge-pilot/internal/shared/secret"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
	commondb "github.com/real-uangi/allingo/common/db"
)

var networkAliasPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var serviceKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,22}[a-z0-9])?$`)

type Service struct {
	repo      domain.Repository
	publisher domain.ProxyConfigPublisher
	agents    domain.AgentLookup
	releases  domain.ReleaseStateChecker
	codec     *secret.Codec
}

func NewService(repo domain.Repository) *Service {
	return NewServiceWithPublisherAndCodecAndReleases(repo, nil, nil, nil, nil)
}

func NewServiceWithPublisher(repo domain.Repository, publisher domain.ProxyConfigPublisher, agents domain.AgentLookup) *Service {
	return NewServiceWithPublisherAndCodecAndReleases(repo, publisher, agents, nil, nil)
}

func NewServiceWithPublisherAndCodec(repo domain.Repository, publisher domain.ProxyConfigPublisher, agents domain.AgentLookup, codec *secret.Codec) *Service {
	return NewServiceWithPublisherAndCodecAndReleases(repo, publisher, agents, codec, nil)
}

func NewServiceWithPublisherAndCodecAndReleases(repo domain.Repository, publisher domain.ProxyConfigPublisher, agents domain.AgentLookup, codec *secret.Codec, releases domain.ReleaseStateChecker) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
		agents:    agents,
		releases:  releases,
		codec:     codec,
	}
}

func (s *Service) Create(req dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
	serviceKey, err := normalizeServiceKey(req.ServiceKey)
	if err != nil {
		return nil, err
	}
	req.ServiceKey = serviceKey
	existing, err := s.repo.GetByKey(req.ServiceKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, business.NewBadRequest("serviceKey 已存在")
	}
	entity, err := s.buildServiceEntity(uuid.New(), req)
	if err != nil {
		return nil, err
	}
	if err := validatePublishedPorts(entity.PublishedPorts.Get()); err != nil {
		return nil, err
	}
	if err := validateTCPProxyPorts(entity.TCPProxyPorts.Get()); err != nil {
		return nil, err
	}
	if err := validateHostPortSets(entity.PublishedPorts.Get(), entity.TCPProxyPorts.Get()); err != nil {
		return nil, err
	}
	if err := s.ensureAgentAssignable(entity.AgentID); err != nil {
		return nil, err
	}
	if err := s.ensureRouteAvailable(entity.AgentID, routeHostsFromService(entity), entity.RoutePathPrefix, entity.ID); err != nil {
		return nil, err
	}
	if err := s.ensureHostPortsAvailable(entity.AgentID, entity.PublishedPorts.Get(), entity.TCPProxyPorts.Get(), entity.ID); err != nil {
		return nil, err
	}
	if err := s.repo.Create(entity); err != nil {
		return nil, err
	}
	if err := s.publishAgent(entity.AgentID); err != nil {
		return nil, err
	}
	output, err := s.toServiceOutput(entity)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) Update(id uuid.UUID, req dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
	serviceKey, err := normalizeServiceKey(req.ServiceKey)
	if err != nil {
		return nil, err
	}
	req.ServiceKey = serviceKey
	current, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, business.ErrNotFound
	}
	if req.ServiceKey != current.ServiceKey {
		return nil, business.NewBadRequest("serviceKey 不允许修改")
	}
	if strings.TrimSpace(req.AgentID) != current.AgentID {
		return nil, business.NewBadRequest("agentId 不允许修改")
	}
	updated, err := s.buildServiceEntity(id, req)
	if err != nil {
		return nil, err
	}
	updated.CreatedAt = current.CreatedAt
	updated.CurrentLiveSlot = current.CurrentLiveSlot
	if err := validatePublishedPorts(updated.PublishedPorts.Get()); err != nil {
		return nil, err
	}
	if err := validateTCPProxyPorts(updated.TCPProxyPorts.Get()); err != nil {
		return nil, err
	}
	if err := validateHostPortSets(updated.PublishedPorts.Get(), updated.TCPProxyPorts.Get()); err != nil {
		return nil, err
	}
	if err := s.ensureAgentAssignable(updated.AgentID); err != nil {
		return nil, err
	}
	if err := s.ensureRouteAvailable(updated.AgentID, routeHostsFromService(updated), updated.RoutePathPrefix, updated.ID); err != nil {
		return nil, err
	}
	if err := s.ensureHostPortsAvailable(updated.AgentID, updated.PublishedPorts.Get(), updated.TCPProxyPorts.Get(), updated.ID); err != nil {
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
	output, err := s.toServiceOutput(updated)
	if err != nil {
		return nil, err
	}
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
	output, err := s.toServiceOutput(service)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) List() ([]dto.ServiceOutput, error) {
	services, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	output := make([]dto.ServiceOutput, 0, len(services))
	for i := range services {
		item, err := s.toServiceOutput(&services[i])
		if err != nil {
			return nil, err
		}
		output = append(output, item)
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
	spec, err := s.toDeploymentSpec(service)
	if err != nil {
		return nil, err
	}
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
	spec, err := s.toDeploymentSpec(service)
	if err != nil {
		return nil, err
	}
	return &spec, nil
}

func (s *Service) UpdateLiveSlot(id uuid.UUID, slot model.Slot) error {
	return s.repo.UpdateLiveSlot(id, slot)
}

func (s *Service) Delete(id uuid.UUID) error {
	current, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if current == nil {
		return business.ErrNotFound
	}
	if err := s.ensureServiceDeletable(id); err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	return s.publishAgent(current.AgentID)
}

func (s *Service) buildServiceEntity(id uuid.UUID, req dto.UpsertServiceRequest) (*model.Service, error) {
	serviceKey, err := normalizeServiceKey(req.ServiceKey)
	if err != nil {
		return nil, err
	}
	if err := validateContainerPort(req.ContainerPort); err != nil {
		return nil, err
	}
	if err := validateResourceLimits(req.CPULimitCores, req.MemoryLimitMB); err != nil {
		return nil, err
	}
	primaryRouteHost, routeHosts, err := normalizeRouteHosts(req.RouteHost, req.RouteHosts)
	if err != nil {
		return nil, err
	}
	normalizedPathPrefix := NormalizeRoutePathPrefix(req.RoutePathPrefix)
	if err := validateRoutePathPrefix(normalizedPathPrefix); err != nil {
		return nil, err
	}
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
		expectedCode = model.DefaultHTTPExpectedCode
	}
	timeoutSeconds := req.HTTPTimeoutSecond
	if timeoutSeconds == 0 {
		timeoutSeconds = model.DefaultHTTPTimeoutSecond
	}
	startupGraceSecond := req.StartupGraceSecond
	if startupGraceSecond == 0 {
		startupGraceSecond = model.DefaultStartupGraceSecond
	}
	httpProbeTimeoutSecond := req.HTTPProbeTimeoutSecond
	if httpProbeTimeoutSecond == 0 {
		httpProbeTimeoutSecond = model.DefaultHTTPProbeTimeoutSecond
	}
	httpProbeIntervalSecond := req.HTTPProbeIntervalSecond
	if httpProbeIntervalSecond == 0 {
		httpProbeIntervalSecond = model.DefaultHTTPProbeIntervalSecond
	}
	httpSuccessThreshold := req.HTTPSuccessThreshold
	if httpSuccessThreshold == 0 {
		httpSuccessThreshold = model.DefaultHTTPSuccessThreshold
	}
	schedulerExecutorGroup := strings.TrimSpace(req.SchedulerExecutorGroup)
	if req.SchedulerSDKPort > 0 && schedulerExecutorGroup == "" {
		schedulerExecutorGroup = "default"
	}
	if err := validateSchedulerSDKConfig(req.SchedulerSDKPort, schedulerExecutorGroup); err != nil {
		return nil, err
	}
	envCiphertext, envKeyVersion, err := s.encryptEnv(req.Env)
	if err != nil {
		return nil, err
	}
	httpHealthHeaders := normalizeHTTPHeaders(req.HTTPHealthHeaders)
	networkAliases, err := normalizeNetworkAliases(req.NetworkAliases)
	if err != nil {
		return nil, err
	}

	return &model.Service{
		ID:                      id,
		ServiceKey:              serviceKey,
		Name:                    req.Name,
		AgentID:                 req.AgentID,
		ImageRepo:               req.ImageRepo,
		ContainerPort:           req.ContainerPort,
		CPULimitCores:           req.CPULimitCores,
		MemoryLimitMB:           req.MemoryLimitMB,
		SchedulerSDKPort:        req.SchedulerSDKPort,
		SchedulerSDKAddr:        req.SchedulerSDKAddr,
		SchedulerExecutorGroup:  schedulerExecutorGroup,
		DockerHealthCheck:       dockerHealth,
		HTTPHealthPath:          req.HTTPHealthPath,
		HTTPHealthHeaders:       commondb.NewJSONB(httpHealthHeaders),
		HTTPExpectedCode:        expectedCode,
		HTTPTimeoutSecond:       timeoutSeconds,
		StartupGraceSecond:      startupGraceSecond,
		HTTPProbeTimeoutSecond:  httpProbeTimeoutSecond,
		HTTPProbeIntervalSecond: httpProbeIntervalSecond,
		HTTPSuccessThreshold:    httpSuccessThreshold,
		RouteHost:               primaryRouteHost,
		RouteHosts:              commondb.NewJSONB(routeHosts),
		RoutePathPrefix:         normalizedPathPrefix,
		Env:                     nil,
		EnvCiphertext:           envCiphertext,
		EnvKeyVersion:           envKeyVersion,
		Command:                 commondb.NewJSONB(req.Command),
		Entrypoint:              commondb.NewJSONB(req.Entrypoint),
		Volumes:                 commondb.NewJSONB(toModelVolumes(req.Volumes)),
		NetworkAliases:          commondb.NewJSONB(networkAliases),
		PublishedPorts:          commondb.NewJSONB(toModelPublishedPorts(req.PublishedPorts)),
		TCPProxyPorts:           commondb.NewJSONB(toModelTCPProxyPorts(req.TCPProxyPorts)),
		Enabled:                 enabled,
	}, nil
}

func normalizeServiceKey(value string) (string, error) {
	serviceKey := strings.TrimSpace(value)
	if !serviceKeyPattern.MatchString(serviceKey) {
		return "", business.NewBadRequest("serviceKey 非法")
	}
	return serviceKey, nil
}

func normalizeRouteHosts(primary string, hosts []string) (string, []string, error) {
	primary = NormalizeRouteHost(primary)
	normalized := make([]string, 0, len(hosts)+1)
	seen := make(map[string]struct{}, len(hosts)+1)
	add := func(value string) {
		host := NormalizeRouteHost(value)
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	add(primary)
	for _, host := range hosts {
		add(host)
	}
	if len(normalized) == 0 {
		return "", nil, business.NewBadRequest("routeHost 必填")
	}
	if primary == "" {
		primary = normalized[0]
	}
	return primary, normalized, nil
}

func routeHostsFromService(service *model.Service) []string {
	if service == nil {
		return nil
	}
	_, hosts, err := normalizeRouteHosts(service.RouteHost, getJSON(service.RouteHosts))
	if err != nil {
		return nil
	}
	return hosts
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

func (s *Service) toServiceOutput(service *model.Service) (dto.ServiceOutput, error) {
	env, err := s.resolveEnv(service)
	if err != nil {
		return dto.ServiceOutput{}, err
	}
	return dto.ServiceOutput{
		ID:                      service.ID,
		Name:                    service.Name,
		ServiceKey:              service.ServiceKey,
		AgentID:                 service.AgentID,
		ImageRepo:               service.ImageRepo,
		ContainerPort:           service.ContainerPort,
		CPULimitCores:           service.CPULimitCores,
		MemoryLimitMB:           service.MemoryLimitMB,
		CurrentLiveSlot:         service.CurrentLiveSlot,
		SchedulerSDKPort:        service.SchedulerSDKPort,
		SchedulerExecutorGroup:  service.SchedulerExecutorGroup,
		DockerHealthCheck:       service.DockerHealthCheck,
		HTTPHealthPath:          service.HTTPHealthPath,
		HTTPHealthHeaders:       getJSON(service.HTTPHealthHeaders),
		HTTPExpectedCode:        service.HTTPExpectedCode,
		HTTPTimeoutSecond:       service.HTTPTimeoutSecond,
		StartupGraceSecond:      service.StartupGraceSecond,
		HTTPProbeTimeoutSecond:  service.HTTPProbeTimeoutSecond,
		HTTPProbeIntervalSecond: service.HTTPProbeIntervalSecond,
		HTTPSuccessThreshold:    service.HTTPSuccessThreshold,
		RouteHost:               service.RouteHost,
		RouteHosts:              routeHostsFromService(service),
		RoutePathPrefix:         service.RoutePathPrefix,
		Env:                     env,
		Command:                 getJSON(service.Command),
		Entrypoint:              getJSON(service.Entrypoint),
		Volumes:                 toDTOVolumes(getJSON(service.Volumes)),
		NetworkAliases:          getJSON(service.NetworkAliases),
		PublishedPorts:          toDTOPublishedPorts(getJSON(service.PublishedPorts)),
		TCPProxyPorts:           toDTOTCPProxyPorts(getJSON(service.TCPProxyPorts)),
		Enabled:                 service.Enabled,
		CreatedAt:               service.CreatedAt,
		UpdatedAt:               service.UpdatedAt,
	}, nil
}

func (s *Service) toDeploymentSpec(service *model.Service) (dto.ServiceDeploymentSpec, error) {
	env, err := s.resolveEnv(service)
	if err != nil {
		return dto.ServiceDeploymentSpec{}, err
	}
	return dto.ServiceDeploymentSpec{
		ID:                      service.ID,
		Name:                    service.Name,
		ServiceKey:              service.ServiceKey,
		AgentID:                 service.AgentID,
		ImageRepo:               service.ImageRepo,
		ContainerPort:           service.ContainerPort,
		CPULimitCores:           service.CPULimitCores,
		MemoryLimitMB:           service.MemoryLimitMB,
		CurrentLiveSlot:         service.CurrentLiveSlot,
		SchedulerSDKPort:        service.SchedulerSDKPort,
		SchedulerSDKAddr:        service.SchedulerSDKAddr,
		SchedulerExecutorGroup:  service.SchedulerExecutorGroup,
		DockerHealthCheck:       service.DockerHealthCheck != nil && *service.DockerHealthCheck,
		HTTPHealthPath:          service.HTTPHealthPath,
		HTTPHealthHeaders:       getJSON(service.HTTPHealthHeaders),
		HTTPExpectedCode:        service.HTTPExpectedCode,
		HTTPTimeoutSecond:       service.HTTPTimeoutSecond,
		StartupGraceSecond:      service.StartupGraceSecond,
		HTTPProbeTimeoutSecond:  service.HTTPProbeTimeoutSecond,
		HTTPProbeIntervalSecond: service.HTTPProbeIntervalSecond,
		HTTPSuccessThreshold:    service.HTTPSuccessThreshold,
		RouteHost:               service.RouteHost,
		RouteHosts:              routeHostsFromService(service),
		RoutePathPrefix:         service.RoutePathPrefix,
		Env:                     env,
		EnvEncrypted:            strings.TrimSpace(service.EnvCiphertext) != "",
		Command:                 getJSON(service.Command),
		Entrypoint:              getJSON(service.Entrypoint),
		Volumes:                 toDTOVolumes(getJSON(service.Volumes)),
		NetworkAliases:          getJSON(service.NetworkAliases),
		PublishedPorts:          toDTOPublishedPorts(getJSON(service.PublishedPorts)),
		TCPProxyPorts:           toDTOTCPProxyPorts(getJSON(service.TCPProxyPorts)),
		Enabled:                 service.Enabled != nil && *service.Enabled,
	}, nil
}

func (s *Service) encryptEnv(env map[string]string) (string, string, error) {
	if len(env) == 0 {
		return "", "", nil
	}
	if s.codec == nil {
		return "", "", business.NewErrorWithCode("service secret master key not configured", 500)
	}
	return s.codec.EncryptJSON(env)
}

func (s *Service) resolveEnv(service *model.Service) (map[string]string, error) {
	if strings.TrimSpace(service.EnvCiphertext) == "" {
		return getJSON(service.Env), nil
	}
	if s.codec == nil {
		return nil, business.NewErrorWithCode("service secret master key not configured", 500)
	}
	var env map[string]string
	if err := s.codec.DecryptJSON(service.EnvCiphertext, service.EnvKeyVersion, &env); err != nil {
		return nil, err
	}
	if env == nil {
		return map[string]string{}, nil
	}
	return env, nil
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

func toModelTCPProxyPorts(items []dto.TCPProxyPort) []model.TCPProxyPort {
	out := make([]model.TCPProxyPort, 0, len(items))
	for _, item := range items {
		idleTimeoutSecond := item.IdleTimeoutSecond
		if idleTimeoutSecond == 0 {
			idleTimeoutSecond = model.DefaultTCPProxyIdleTimeoutSec
		}
		out = append(out, model.TCPProxyPort{
			ListenPort:        item.ListenPort,
			ContainerPort:     item.ContainerPort,
			IdleTimeoutSecond: idleTimeoutSecond,
		})
	}
	return out
}

func toDTOTCPProxyPorts(items []model.TCPProxyPort) []dto.TCPProxyPort {
	out := make([]dto.TCPProxyPort, 0, len(items))
	for _, item := range items {
		out = append(out, dto.TCPProxyPort{
			ListenPort:        item.ListenPort,
			ContainerPort:     item.ContainerPort,
			IdleTimeoutSecond: item.IdleTimeoutSecond,
		})
	}
	return out
}

func boolPointer(v bool) *bool {
	return &v
}

func normalizeHTTPHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(headers))
	for key, value := range headers {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		normalized[trimmedKey] = trimmedValue
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeNetworkAliases(items []string) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		alias := strings.TrimSpace(item)
		if alias == "" {
			continue
		}
		if !networkAliasPattern.MatchString(alias) {
			return nil, business.NewBadRequest("networkAliases 非法")
		}
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		normalized = append(normalized, alias)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
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
		if isReservedProxyPort(item.HostPort) {
			return business.NewBadRequest("publishedPorts.hostPort 与代理保留端口冲突")
		}
		if _, ok := seen[item.HostPort]; ok {
			return business.NewBadRequest("publishedPorts.hostPort 重复")
		}
		seen[item.HostPort] = struct{}{}
	}
	return nil
}

func validateTCPProxyPorts(items []model.TCPProxyPort) error {
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		if item.ListenPort <= 0 || item.ListenPort > 65535 {
			return business.NewBadRequest("tcpProxyPorts.listenPort 非法")
		}
		if item.ContainerPort <= 0 || item.ContainerPort > 65535 {
			return business.NewBadRequest("tcpProxyPorts.containerPort 非法")
		}
		if item.IdleTimeoutSecond <= 0 {
			return business.NewBadRequest("tcpProxyPorts.idleTimeoutSecond 非法")
		}
		if isReservedProxyPort(item.ListenPort) {
			return business.NewBadRequest("tcpProxyPorts.listenPort 与代理保留端口冲突")
		}
		if _, ok := seen[item.ListenPort]; ok {
			return business.NewBadRequest("tcpProxyPorts.listenPort 重复")
		}
		seen[item.ListenPort] = struct{}{}
	}
	return nil
}

func validateHostPortSets(publishedPorts []model.PublishedPort, tcpProxyPorts []model.TCPProxyPort) error {
	published := make(map[int]struct{}, len(publishedPorts))
	for _, item := range publishedPorts {
		published[item.HostPort] = struct{}{}
	}
	for _, item := range tcpProxyPorts {
		if _, ok := published[item.ListenPort]; ok {
			return business.NewBadRequest("tcpProxyPorts.listenPort 与 publishedPorts.hostPort 冲突")
		}
	}
	return nil
}

func isReservedProxyPort(port int) bool {
	return port == SharedFrontendBindPort || port == 5555 || port == 19999
}

func validateContainerPort(port int) error {
	if port <= 0 || port > 65535 {
		return business.NewBadRequest("containerPort 非法")
	}
	return nil
}

func validateResourceLimits(cpuLimitCores float64, memoryLimitMB int64) error {
	if cpuLimitCores < 0 {
		return business.NewBadRequest("cpuLimitCores 非法")
	}
	if memoryLimitMB < 0 {
		return business.NewBadRequest("memoryLimitMB 非法")
	}
	return nil
}

func validateSchedulerSDKConfig(port int, group string) error {
	if port < 0 || port > 65535 {
		return business.NewBadRequest("schedulerSdkPort 非法")
	}
	if port > 0 && strings.TrimSpace(group) == "" {
		return business.NewBadRequest("schedulerExecutorGroup 必填")
	}
	return nil
}

func validateRoutePathPrefix(path string) error {
	for _, r := range path {
		if r <= 0x1f || r == 0x7f || r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return business.NewBadRequest("routePathPrefix 非法")
		}
		if r == ';' || r == '?' || r == '#' {
			return business.NewBadRequest("routePathPrefix 非法")
		}
	}
	return nil
}

func (s *Service) ensureRouteAvailable(agentID string, routeHosts []string, routePathPrefix string, selfID uuid.UUID) error {
	if len(routeHosts) == 0 {
		return business.NewBadRequest("routeHost 必填")
	}
	services, err := s.repo.ListByAgent(agentID)
	if err != nil {
		return err
	}
	requested := make(map[string]struct{}, len(routeHosts))
	for _, host := range routeHosts {
		requested[host] = struct{}{}
	}
	for i := range services {
		if services[i].ID == selfID || NormalizeRoutePathPrefix(services[i].RoutePathPrefix) != routePathPrefix {
			continue
		}
		for _, host := range routeHostsFromService(&services[i]) {
			if _, ok := requested[host]; ok {
				return business.NewBadRequest("routeHost + routePathPrefix 已存在")
			}
		}
	}
	return nil
}

func (s *Service) ensureHostPortsAvailable(agentID string, publishedPorts []model.PublishedPort, tcpProxyPorts []model.TCPProxyPort, selfID uuid.UUID) error {
	if len(publishedPorts) == 0 && len(tcpProxyPorts) == 0 {
		return nil
	}
	services, err := s.repo.ListByAgent(agentID)
	if err != nil {
		return err
	}
	requested := make(map[int]struct{}, len(publishedPorts)+len(tcpProxyPorts))
	for _, port := range publishedPorts {
		requested[port.HostPort] = struct{}{}
	}
	for _, port := range tcpProxyPorts {
		requested[port.ListenPort] = struct{}{}
	}
	for i := range services {
		if services[i].ID == selfID {
			continue
		}
		for _, port := range getJSON(services[i].PublishedPorts) {
			if _, ok := requested[port.HostPort]; ok {
				return business.NewBadRequest(fmt.Sprintf("host port 已被服务 %s 占用", services[i].ServiceKey))
			}
		}
		for _, port := range getJSON(services[i].TCPProxyPorts) {
			if _, ok := requested[port.ListenPort]; ok {
				return business.NewBadRequest(fmt.Sprintf("host port 已被服务 %s 占用", services[i].ServiceKey))
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

func (s *Service) ensureServiceDeletable(serviceID uuid.UUID) error {
	if s.releases == nil {
		return nil
	}
	active, err := s.releases.HasActiveRelease(serviceID)
	if err != nil {
		return err
	}
	if active {
		return business.NewErrorWithCode("服务存在进行中的发布，禁止删除", 409)
	}
	split, err := s.releases.HasTrafficSplitRelease(serviceID)
	if err != nil {
		return err
	}
	if split {
		return business.NewErrorWithCode("服务存在进行中的发布，禁止删除", 409)
	}
	return nil
}

func getJSON[T any](value *commondb.JSONB[T]) T {
	var zero T
	if value == nil {
		return zero
	}
	return value.Get()
}
