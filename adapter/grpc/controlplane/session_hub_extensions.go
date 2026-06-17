package controlplane

import (
	"context"
	"fmt"
	"sync"

	releasedomain "github.com/real-uangi/edge-pilot/internal/release/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"

	"github.com/google/uuid"
)

func (h *sessionHub) RequestContainerList(ctx context.Context, agentID string) ([]*grpcapi.ContainerSummary, error) {
	requestID := uuid.NewString()
	responseCh := make(chan *grpcapi.ContainerListResponse, 1)

	h.pendingMu.Lock()
	h.pendingListResponses[h.pendingConfigRequestKey(agentID, requestID)] = responseCh
	h.pendingMu.Unlock()

	key := h.pendingConfigRequestKey(agentID, requestID)
	defer func() {
		h.pendingMu.Lock()
		delete(h.pendingListResponses, key)
		h.pendingMu.Unlock()
	}()

	session, ok := h.getSession(agentID)
	if !ok {
		return nil, releasedomain.ErrAgentOffline
	}

	if err := session.send(&grpcapi.ControlMessage{
		Payload: &grpcapi.ControlMessage_ContainerListRequest{
			ContainerListRequest: &grpcapi.ContainerListRequest{
				RequestId: requestID,
				AgentId:   agentID,
			},
		},
	}); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-responseCh:
		if response == nil {
			return nil, releasedomain.ErrAgentOffline
		}
		if response.GetErrorMessage() != "" {
			return nil, fmt.Errorf("agent error: %s", response.GetErrorMessage())
		}
		return response.GetContainers(), nil
	}
}

func (h *sessionHub) RequestContainerInspect(ctx context.Context, agentID, containerID string) (*grpcapi.ContainerDetails, error) {
	requestID := uuid.NewString()
	responseCh := make(chan *grpcapi.ContainerInspectResponse, 1)

	key := h.pendingConfigRequestKey(agentID, requestID)
	h.pendingMu.Lock()
	h.pendingInspectResponses[key] = responseCh
	h.pendingMu.Unlock()
	defer func() {
		h.pendingMu.Lock()
		delete(h.pendingInspectResponses, key)
		h.pendingMu.Unlock()
	}()

	session, ok := h.getSession(agentID)
	if !ok {
		return nil, releasedomain.ErrAgentOffline
	}

	if err := session.send(&grpcapi.ControlMessage{
		Payload: &grpcapi.ControlMessage_ContainerInspectRequest{
			ContainerInspectRequest: &grpcapi.ContainerInspectRequest{
				RequestId:   requestID,
				AgentId:     agentID,
				ContainerId: containerID,
			},
		},
	}); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-responseCh:
		if response == nil {
			return nil, releasedomain.ErrAgentOffline
		}
		if response.GetErrorMessage() != "" {
			return nil, fmt.Errorf("agent error: %s", response.GetErrorMessage())
		}
		return response.GetDetails(), nil
	}
}

func (h *sessionHub) resolveContainerListResponse(response *grpcapi.ContainerListResponse) {
	if response == nil {
		return
	}
	key := h.pendingConfigRequestKey(response.GetAgentId(), response.GetRequestId())

	h.pendingMu.Lock()
	ch := h.pendingListResponses[key]
	delete(h.pendingListResponses, key)
	h.pendingMu.Unlock()

	if ch == nil {
		return
	}
	select {
	case ch <- response:
	default:
	}
}

func (h *sessionHub) resolveContainerInspectResponse(response *grpcapi.ContainerInspectResponse) {
	if response == nil {
		return
	}
	key := h.pendingConfigRequestKey(response.GetAgentId(), response.GetRequestId())

	h.pendingMu.Lock()
	ch := h.pendingInspectResponses[key]
	delete(h.pendingInspectResponses, key)
	h.pendingMu.Unlock()

	if ch == nil {
		return
	}
	select {
	case ch <- response:
	default:
	}
}

// LogStreamManager 管理活跃的日志流
type LogStreamManager struct {
	mu      sync.RWMutex
	streams map[string]chan *grpcapi.ContainerLogChunk
}

func NewLogStreamManager() *LogStreamManager {
	return &LogStreamManager{
		streams: make(map[string]chan *grpcapi.ContainerLogChunk),
	}
}

func (m *LogStreamManager) Register(agentID, containerID string) chan *grpcapi.ContainerLogChunk {
	key := agentID + ":" + containerID
	ch := make(chan *grpcapi.ContainerLogChunk, 64)
	m.mu.Lock()
	m.streams[key] = ch
	m.mu.Unlock()
	return ch
}

func (m *LogStreamManager) Unregister(agentID, containerID string) {
	key := agentID + ":" + containerID
	m.mu.Lock()
	if ch, ok := m.streams[key]; ok {
		close(ch)
		delete(m.streams, key)
	}
	m.mu.Unlock()
}

func (m *LogStreamManager) UnregisterByAgent(agentID string) {
	prefix := agentID + ":"
	m.mu.RLock()
	var keys []string
	for key := range m.streams {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	for _, key := range keys {
		if ch, ok := m.streams[key]; ok {
			close(ch)
			delete(m.streams, key)
		}
	}
	m.mu.Unlock()
}

func (m *LogStreamManager) ForwardChunk(chunk *grpcapi.ContainerLogChunk) bool {
	key := chunk.GetAgentId() + ":" + chunk.GetContainerId()
	m.mu.RLock()
	ch, ok := m.streams[key]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	defer func() {
		recover()
	}()
	select {
	case ch <- chunk:
		return true
	default:
		return false
	}
}

func (h *sessionHub) StartContainerLogStream(ctx context.Context, agentID, containerID string, tailLines int32) (chan *grpcapi.ContainerLogChunk, error) {
	session, ok := h.getSession(agentID)
	if !ok {
		return nil, releasedomain.ErrAgentOffline
	}

	requestID := uuid.NewString()
	ch := h.logStreams.Register(agentID, containerID)

	if err := session.send(&grpcapi.ControlMessage{
		Payload: &grpcapi.ControlMessage_ContainerLogStreamRequest{
			ContainerLogStreamRequest: &grpcapi.ContainerLogStreamRequest{
				RequestId:   requestID,
				AgentId:     agentID,
				ContainerId: containerID,
				Follow:      true,
				TailLines:   tailLines,
			},
		},
	}); err != nil {
		h.logStreams.Unregister(agentID, containerID)
		return nil, err
	}

	return ch, nil
}

func (h *sessionHub) StopContainerLogStream(agentID, containerID string) error {
	h.logStreams.Unregister(agentID, containerID)

	session, ok := h.getSession(agentID)
	if !ok {
		return nil // agent 已离线，无需发送停止
	}

	return session.send(&grpcapi.ControlMessage{
		Payload: &grpcapi.ControlMessage_ContainerLogStreamRequest{
			ContainerLogStreamRequest: &grpcapi.ContainerLogStreamRequest{
				RequestId:   uuid.NewString(),
				AgentId:     agentID,
				ContainerId: containerID,
				Follow:      false,
			},
		},
	})
}
