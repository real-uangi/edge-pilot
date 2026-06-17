package proxyconfig

import (
	"context"
	"errors"
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/dto"

	"github.com/real-uangi/allingo/common/business"
)

var ErrHAProxyConfigTimeout = errors.New("haproxy config request timeout")

type HAProxyConfigRequester interface {
	RequestHAProxyConfig(ctx context.Context, agentID string) (string, error)
}

type AgentOnlineChecker interface {
	IsOnline(agentID string) (bool, error)
}

type HAProxyConfigService struct {
	agents    AgentOnlineChecker
	requester HAProxyConfigRequester
}

func NewHAProxyConfigService(agents AgentOnlineChecker, requester HAProxyConfigRequester) *HAProxyConfigService {
	return &HAProxyConfigService{
		agents:    agents,
		requester: requester,
	}
}

func (s *HAProxyConfigService) GetHAProxyConfig(agentID string) (*dto.AgentHAProxyConfigOutput, error) {
	online, err := s.agents.IsOnline(agentID)
	if err != nil {
		return nil, err
	}
	if !online {
		return nil, business.NewErrorWithCode("节点离线，无法获取实时配置", 409)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	configText, err := s.requester.RequestHAProxyConfig(ctx, agentID)
	if err != nil {
		if errors.Is(err, ErrHAProxyConfigTimeout) {
			return nil, business.NewErrorWithCode("获取超时，请重试", 504)
		}
		return nil, err
	}
	return &dto.AgentHAProxyConfigOutput{
		AgentID:   agentID,
		Config:    configText,
		FetchedAt: time.Now(),
	}, nil
}
