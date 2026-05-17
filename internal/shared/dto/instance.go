package dto

type ManagedInstanceOutput struct {
	AgentID     string `json:"agentId"`
	AgentHost   string `json:"agentHost"`
	ContainerID string `json:"containerId"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Image       string `json:"image"`
	ServiceID   string `json:"serviceId"`
	ServiceKey  string `json:"serviceKey"`
	ReleaseID   string `json:"releaseId"`
	Slot        string `json:"slot"`
	CreatedAt   int64  `json:"createdAt"`
}

type ManagedInstanceDetailsOutput struct {
	ManagedInstanceOutput
	Running      bool              `json:"running"`
	Health       string            `json:"health"`
	RestartCount int               `json:"restartCount"`
	IPAddress    string            `json:"ipAddress"`
	Labels       map[string]string `json:"labels"`
	Env          map[string]string `json:"env"`
	Command      []string          `json:"command"`
	Entrypoint   []string          `json:"entrypoint"`
	Volumes      []VolumeMount     `json:"volumes"`
	Ports        []PublishedPort   `json:"ports"`
	CPULimit     float64           `json:"cpuLimit"`
	MemoryLimit  int64             `json:"memoryLimit"`
}
