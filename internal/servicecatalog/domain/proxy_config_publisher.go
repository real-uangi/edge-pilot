package domain

type ProxyConfigPublisher interface {
	PublishAgent(agentID string) error
}
