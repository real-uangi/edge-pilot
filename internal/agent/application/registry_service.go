package application

import (
	"edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"time"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
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

func (s *RegistryService) Authenticate(agentID string, token string) error {
	if _, err := uuid.Parse(agentID); err != nil {
		return business.ErrUnauthorized
	}
	node, err := s.repo.Get(agentID)
	if err != nil {
		return err
	}
	if node == nil || node.Enabled == nil || !*node.Enabled {
		return business.ErrUnauthorized
	}
	if !s.auth.ValidateHash(node.TokenHash, token) {
		return business.ErrUnauthorized
	}
	return nil
}

func (s *RegistryService) CreateAgent() (*dto.AgentCredentialOutput, error) {
	token, hash, err := s.auth.GenerateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	enabled := true
	online := false
	node := &model.AgentNode{
		ID:             uuid.NewString(),
		TokenHash:      hash,
		Enabled:        &enabled,
		Online:         &online,
		TokenRotatedAt: &now,
	}
	if err := s.repo.Save(node); err != nil {
		return nil, err
	}
	output := toAgentCredentialOutput(node, token)
	return &output, nil
}

func (s *RegistryService) ResetToken(agentID string) (*dto.AgentCredentialOutput, error) {
	node, err := s.repo.Get(agentID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, business.ErrNotFound
	}
	token, hash, err := s.auth.GenerateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	node.TokenHash = hash
	node.TokenRotatedAt = &now
	if err := s.repo.Save(node); err != nil {
		return nil, err
	}
	output := toAgentCredentialOutput(node, token)
	return &output, nil
}

func (s *RegistryService) Enable(agentID string) (*dto.AgentOutput, error) {
	return s.setEnabled(agentID, true)
}

func (s *RegistryService) Disable(agentID string) (*dto.AgentOutput, error) {
	return s.setEnabled(agentID, false)
}

func (s *RegistryService) GetAgent(agentID string) (*dto.AgentOutput, error) {
	node, err := s.repo.Get(agentID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, business.ErrNotFound
	}
	output := toAgentOutput(node)
	return &output, nil
}

func (s *RegistryService) MarkConnected(agentID string, hostname string, version string, capabilities []string) error {
	node, err := s.repo.Get(agentID)
	if err != nil {
		return err
	}
	if node == nil {
		return business.ErrNotFound
	}
	now := time.Now()
	online := true
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
		return business.ErrNotFound
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

func (s *RegistryService) MarkOfflineStale(before time.Time) ([]string, error) {
	return s.repo.MarkOfflineStale(before)
}

func (s *RegistryService) IsOnline(agentID string) (bool, error) {
	node, err := s.repo.Get(agentID)
	if err != nil {
		return false, err
	}
	if node == nil || node.Online == nil || node.Enabled == nil {
		return false, nil
	}
	return *node.Online && *node.Enabled, nil
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
			Enabled:         nodes[i].Enabled,
			Hostname:        nodes[i].Hostname,
			Version:         nodes[i].Version,
			Online:          nodes[i].Online,
			LastHeartbeatAt: nodes[i].LastHeartbeatAt,
		})
	}
	return output, nil
}

func (s *RegistryService) ListAgents() ([]dto.AgentOutput, error) {
	nodes, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	output := make([]dto.AgentOutput, 0, len(nodes))
	for i := range nodes {
		output = append(output, toAgentOutput(&nodes[i]))
	}
	return output, nil
}

func (s *RegistryService) setEnabled(agentID string, enabled bool) (*dto.AgentOutput, error) {
	node, err := s.repo.Get(agentID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, business.ErrNotFound
	}
	node.Enabled = boolPointer(enabled)
	if err := s.repo.Save(node); err != nil {
		return nil, err
	}
	output := toAgentOutput(node)
	return &output, nil
}

func toAgentOutput(node *model.AgentNode) dto.AgentOutput {
	return dto.AgentOutput{
		ID:              node.ID,
		Enabled:         node.Enabled,
		Hostname:        node.Hostname,
		Version:         node.Version,
		Online:          node.Online,
		LastHeartbeatAt: node.LastHeartbeatAt,
		LastConnectedAt: node.LastConnectedAt,
		LastError:       node.LastError,
		TokenRotatedAt:  node.TokenRotatedAt,
		CreatedAt:       node.CreatedAt,
		UpdatedAt:       node.UpdatedAt,
	}
}

func toAgentCredentialOutput(node *model.AgentNode, token string) dto.AgentCredentialOutput {
	return dto.AgentCredentialOutput{
		ID:              node.ID,
		Token:           token,
		Enabled:         node.Enabled,
		Hostname:        node.Hostname,
		Version:         node.Version,
		Online:          node.Online,
		LastHeartbeatAt: node.LastHeartbeatAt,
		LastConnectedAt: node.LastConnectedAt,
		LastError:       node.LastError,
		TokenRotatedAt:  node.TokenRotatedAt,
		CreatedAt:       node.CreatedAt,
		UpdatedAt:       node.UpdatedAt,
	}
}

func boolPointer(v bool) *bool {
	return &v
}
