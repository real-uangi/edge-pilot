package application

import (
	"edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"time"

	commondb "github.com/real-uangi/allingo/common/db"
)

type RegistryService struct {
	auth *config.AgentAuthConfig
	repo domain.Repository
}

func NewRegistryService(auth *config.AgentAuthConfig, repo domain.Repository) *RegistryService {
	return &RegistryService{
		auth: auth,
		repo: repo,
	}
}

func (s *RegistryService) Authenticate(agentID string, token string) bool {
	return s.auth.Validate(agentID, token)
}

func (s *RegistryService) MarkConnected(agentID string, hostname string, version string, capabilities []string) error {
	now := time.Now()
	online := true
	node, err := s.repo.Get(agentID)
	if err != nil {
		return err
	}
	if node == nil {
		node = &model.AgentNode{ID: agentID}
	}
	node.Hostname = hostname
	node.Version = version
	node.Online = &online
	node.LastConnectedAt = &now
	node.LastHeartbeatAt = &now
	node.Capabilities = commondb.NewJSONB(capabilities)
	node.LastError = ""
	return s.repo.Save(node)
}

func (s *RegistryService) Heartbeat(agentID string) error {
	node, err := s.repo.Get(agentID)
	if err != nil {
		return err
	}
	if node == nil {
		now := time.Now()
		online := true
		node = &model.AgentNode{
			ID:              agentID,
			Online:          &online,
			LastConnectedAt: &now,
			LastHeartbeatAt: &now,
		}
		return s.repo.Save(node)
	}
	now := time.Now()
	online := true
	node.Online = &online
	node.LastHeartbeatAt = &now
	return s.repo.Save(node)
}

func (s *RegistryService) MarkDisconnected(agentID string, reason string) error {
	return s.repo.MarkOffline(agentID, reason)
}

func (s *RegistryService) IsOnline(agentID string) (bool, error) {
	node, err := s.repo.Get(agentID)
	if err != nil {
		return false, err
	}
	if node == nil || node.Online == nil {
		return false, nil
	}
	return *node.Online, nil
}

func (s *RegistryService) List() ([]dto.AgentOverview, error) {
	nodes, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	output := make([]dto.AgentOverview, 0, len(nodes))
	for i := range nodes {
		output = append(output, dto.AgentOverview{
			ID:              nodes[i].ID,
			Hostname:        nodes[i].Hostname,
			Version:         nodes[i].Version,
			Online:          nodes[i].Online,
			LastHeartbeatAt: nodes[i].LastHeartbeatAt,
		})
	}
	return output, nil
}
