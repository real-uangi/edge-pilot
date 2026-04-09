package grpcapi

type AgentMessage struct {
	Kind       string            `json:"kind"`
	Hello      *HelloMessage     `json:"hello,omitempty"`
	Heartbeat  *HeartbeatMessage `json:"heartbeat,omitempty"`
	TaskUpdate *TaskUpdate       `json:"taskUpdate,omitempty"`
	Stats      *StatsReport      `json:"stats,omitempty"`
}

type ControlMessage struct {
	Kind string       `json:"kind"`
	Ack  *AckMessage  `json:"ack,omitempty"`
	Task *TaskCommand `json:"task,omitempty"`
}

type HelloMessage struct {
	AgentID      string   `json:"agentId"`
	Token        string   `json:"token"`
	Version      string   `json:"version"`
	Hostname     string   `json:"hostname"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type HeartbeatMessage struct {
	AgentID        string   `json:"agentId"`
	RunningTaskIDs []string `json:"runningTaskIds,omitempty"`
}

type AckMessage struct {
	Message string `json:"message"`
}

type TaskCommand struct {
	TaskID            string            `json:"taskId"`
	ReleaseID         string            `json:"releaseId"`
	ServiceID         string            `json:"serviceId"`
	ServiceKey        string            `json:"serviceKey"`
	AgentID           string            `json:"agentId"`
	Type              string            `json:"type"`
	ImageRepo         string            `json:"imageRepo"`
	ImageTag          string            `json:"imageTag"`
	CommitSHA         string            `json:"commitSha"`
	TraceID           string            `json:"traceId"`
	TargetSlot        int               `json:"targetSlot"`
	CurrentLiveSlot   int               `json:"currentLiveSlot"`
	ContainerPort     int               `json:"containerPort"`
	HostPort          int               `json:"hostPort"`
	HTTPHealthPath    string            `json:"httpHealthPath"`
	HTTPExpectedCode  int               `json:"httpExpectedCode"`
	HTTPTimeoutSecond int               `json:"httpTimeoutSecond"`
	BackendName       string            `json:"backendName"`
	ServerName        string            `json:"serverName"`
	PreviousServer    string            `json:"previousServer"`
	Env               map[string]string `json:"env,omitempty"`
	Command           []string          `json:"command,omitempty"`
	Entrypoint        []string          `json:"entrypoint,omitempty"`
	Volumes           []VolumeMount     `json:"volumes,omitempty"`
}

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

type TaskUpdate struct {
	TaskID        string           `json:"taskId"`
	Status        string           `json:"status"`
	Step          string           `json:"step"`
	ErrorMessage  string           `json:"errorMessage,omitempty"`
	ContainerID   string           `json:"containerId,omitempty"`
	ListenAddress string           `json:"listenAddress,omitempty"`
	Slot          int              `json:"slot,omitempty"`
	ServerName    string           `json:"serverName,omitempty"`
	Metrics       map[string]int64 `json:"metrics,omitempty"`
}

type StatsReport struct {
	AgentID  string             `json:"agentId"`
	Services []BackendStatPoint `json:"services,omitempty"`
}

type BackendStatPoint struct {
	ServiceID     string `json:"serviceId"`
	BackendName   string `json:"backendName"`
	ServerName    string `json:"serverName"`
	Scur          int64  `json:"scur"`
	Rate          int64  `json:"rate"`
	ErrorRequests int64  `json:"errorRequests"`
}
