package infra

import (
	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	servicecatalogdomain "github.com/real-uangi/edge-pilot/internal/servicecatalog/domain"
)

type agentServiceBindingChecker struct {
	repo servicecatalogdomain.Repository
}

func NewAgentServiceBindingChecker(repo servicecatalogdomain.Repository) agentdomain.ServiceBindingChecker {
	return &agentServiceBindingChecker{repo: repo}
}

func (c *agentServiceBindingChecker) CountByAgent(agentID string) (int, error) {
	services, err := c.repo.ListByAgent(agentID)
	if err != nil {
		return 0, err
	}
	return len(services), nil
}
