package controlplane

import (
	schedulerapp "edge-pilot/internal/scheduler/application"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"encoding/json"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type schedulerChannelType int

const (
	schedulerChannelDirect schedulerChannelType = iota + 1
	schedulerChannelRelay
)

type schedulerSessionKey struct {
	executorID     string
	channelType    schedulerChannelType
	relaySessionID string
}

type schedulerSessionHub struct {
	mu       sync.RWMutex
	sessions map[schedulerSessionKey]*executorSession
}

type executorSession struct {
	mu              sync.Mutex
	executorID      string
	group           string
	liveSlot        model.Slot
	channelType     schedulerChannelType
	relaySessionID  string
	agentID         string
	routingKey      string
	serviceInstance bool
	serviceID       string
	sendCh          chan *grpcapi.SchedulerMessage
	closed          bool
	agentHub        *sessionHub
}

const schedulerServiceInstanceMetadataKey = "edge_pilot_service_instance"

func NewSchedulerSessionHub() *schedulerSessionHub {
	return &schedulerSessionHub{sessions: make(map[schedulerSessionKey]*executorSession)}
}

func (h *schedulerSessionHub) registerDirect(executorID string, group string, liveSlot model.Slot) *executorSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := schedulerSessionKey{executorID: executorID, channelType: schedulerChannelDirect}
	if existing, ok := h.sessions[key]; ok {
		existing.close()
	}
	session := &executorSession{
		executorID:  executorID,
		group:       group,
		liveSlot:    liveSlot,
		channelType: schedulerChannelDirect,
		sendCh:      make(chan *grpcapi.SchedulerMessage, 16),
	}
	h.sessions[key] = session
	return session
}

func (h *schedulerSessionHub) registerRelay(executorID string, group string, liveSlot model.Slot, agentID string, relaySessionID string, routingKey string, serviceInstance bool, serviceID string, agentHub *sessionHub) *executorSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := schedulerSessionKey{executorID: executorID, channelType: schedulerChannelRelay, relaySessionID: relaySessionID}
	if existing, ok := h.sessions[key]; ok {
		existing.close()
	}
	session := &executorSession{
		executorID:      executorID,
		group:           group,
		liveSlot:        liveSlot,
		channelType:     schedulerChannelRelay,
		relaySessionID:  relaySessionID,
		agentID:         agentID,
		routingKey:      routingKey,
		serviceInstance: serviceInstance,
		serviceID:       strings.TrimSpace(serviceID),
		agentHub:        agentHub,
	}
	h.sessions[key] = session
	return session
}

func (h *schedulerSessionHub) unregisterDirect(executorID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := schedulerSessionKey{executorID: executorID, channelType: schedulerChannelDirect}
	if s, ok := h.sessions[key]; ok {
		s.close()
		delete(h.sessions, key)
	}
}

func (h *schedulerSessionHub) unregisterRelay(agentID string, relaySessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, session := range h.sessions {
		if key.channelType != schedulerChannelRelay {
			continue
		}
		if session.relaySessionID != relaySessionID {
			continue
		}
		if agentID != "" && session.agentID != agentID {
			continue
		}
		session.close()
		delete(h.sessions, key)
	}
}

func (h *schedulerSessionHub) unregisterRelayByAgent(agentID string) {
	if agentID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, session := range h.sessions {
		if key.channelType != schedulerChannelRelay {
			continue
		}
		if session.agentID != agentID {
			continue
		}
		session.close()
		delete(h.sessions, key)
	}
}

func (h *schedulerSessionHub) getRelay(relaySessionID string, agentID string) *executorSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, session := range h.sessions {
		if session.channelType != schedulerChannelRelay {
			continue
		}
		if session.relaySessionID != relaySessionID {
			continue
		}
		if agentID != "" && session.agentID != agentID {
			continue
		}
		return session
	}
	return nil
}

func (h *schedulerSessionHub) preferredForDispatch(executorID string) *executorSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	directKey := schedulerSessionKey{executorID: executorID, channelType: schedulerChannelDirect}
	if session, ok := h.sessions[directKey]; ok {
		return session
	}
	for _, session := range h.sessions {
		if session.executorID == executorID {
			return session
		}
	}
	return nil
}

func (h *schedulerSessionHub) ListOnlineExecutors(group string) []schedulerapp.OnlineExecutor {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[string]struct{})
	out := make([]schedulerapp.OnlineExecutor, 0)
	for _, session := range h.sessions {
		if session.channelType != schedulerChannelDirect {
			continue
		}
		if group != "" && session.group != group {
			continue
		}
		if _, ok := seen[session.executorID]; ok {
			continue
		}
		seen[session.executorID] = struct{}{}
		out = append(out, schedulerapp.OnlineExecutor{
			ExecutorID:      session.executorID,
			Group:           session.group,
			LiveSlot:        session.liveSlot,
			ServiceInstance: session.serviceInstance,
			ServiceID:       session.serviceID,
		})
	}
	for _, session := range h.sessions {
		if session.channelType != schedulerChannelRelay {
			continue
		}
		if group != "" && session.group != group {
			continue
		}
		if _, ok := seen[session.executorID]; ok {
			continue
		}
		seen[session.executorID] = struct{}{}
		out = append(out, schedulerapp.OnlineExecutor{
			ExecutorID:      session.executorID,
			Group:           session.group,
			LiveSlot:        session.liveSlot,
			ServiceInstance: session.serviceInstance,
			ServiceID:       session.serviceID,
		})
	}
	return out
}

func (h *schedulerSessionHub) DispatchRun(executorID string, run *model.SchedulerJobRun) error {
	session := h.preferredForDispatch(executorID)
	if session == nil {
		return status.Error(codes.Unavailable, "executor offline")
	}
	payload := map[string]any{}
	if run.Payload != nil {
		payload = run.Payload.Get()
	}
	payloadBytes, _ := json.Marshal(payload)
	return session.send(&grpcapi.SchedulerMessage{
		Payload: &grpcapi.SchedulerMessage_Run{
			Run: &grpcapi.SchedulerRunCommand{
				RunId:          run.ID.String(),
				JobId:          run.JobID.String(),
				HandlerKey:     run.HandlerKey,
				IdempotencyKey: run.IdempotencyKey,
				Attempt:        int32(run.Attempt),
				PayloadJson:    string(payloadBytes),
			},
		},
	})
}

type SchedulerServer struct {
	grpcapi.UnimplementedSchedulerControlServer

	hub      *schedulerSessionHub
	agentHub *sessionHub
	service  *schedulerapp.Service
	logger   *log.StdLogger
}

func NewSchedulerServer(hub *schedulerSessionHub, agentHub *sessionHub, service *schedulerapp.Service) *SchedulerServer {
	return &SchedulerServer{
		hub:      hub,
		agentHub: agentHub,
		service:  service,
		logger:   log.NewStdLogger("grpc.scheduler"),
	}
}

func (s *SchedulerServer) Connect(stream grpcapi.SchedulerControl_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil {
		return status.Error(codes.InvalidArgument, "hello required")
	}
	executorID := hello.GetExecutorId()
	executor, err := s.service.AuthenticateExecutor(
		executorID,
		hello.GetToken(),
		hello.GetGroup(),
		model.Slot(hello.GetLiveSlot()),
		hello.GetMetadata(),
		"",
		"",
	)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	session := s.hub.registerDirect(executorID, executor.Group, executor.LiveSlot)
	defer s.hub.unregisterDirect(executorID)

	sendErrCh := make(chan error, 1)
	go func() {
		for msg := range session.sendCh {
			if sendErr := stream.Send(msg); sendErr != nil {
				sendErrCh <- sendErr
				return
			}
		}
	}()

	if err := stream.Send(&grpcapi.SchedulerMessage{Payload: &grpcapi.SchedulerMessage_Ack{Ack: &grpcapi.SchedulerAck{Message: "connected"}}}); err != nil {
		return err
	}

	for {
		select {
		case sendErr := <-sendErrCh:
			return sendErr
		default:
		}
		msg, recvErr := stream.Recv()
		if recvErr != nil {
			return recvErr
		}
		s.handleExecutorInbound(executorID, msg)
	}
}

func (s *SchedulerServer) HandleRelayEnvelope(agentID string, envelope *grpcapi.SchedulerRelayEnvelope) error {
	if envelope == nil {
		return nil
	}
	if envelope.GetAgentId() != "" && envelope.GetAgentId() != agentID {
		return status.Error(codes.PermissionDenied, "relay agent id mismatch")
	}
	switch envelope.GetPayloadType() {
	case grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_EXECUTOR_MESSAGE:
		var executorMsg grpcapi.ExecutorMessage
		if err := proto.Unmarshal(envelope.GetPayloadBytes(), &executorMsg); err != nil {
			return err
		}
		return s.handleRelayExecutorMessage(agentID, envelope, &executorMsg)
	case grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SESSION_CLOSED,
		grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SESSION_ERROR:
		s.hub.unregisterRelay(agentID, envelope.GetRelaySessionId())
		return nil
	default:
		return nil
	}
}

func (s *SchedulerServer) CleanupAgentSessions(agentID string) {
	s.hub.unregisterRelayByAgent(agentID)
}

func (s *SchedulerServer) handleRelayExecutorMessage(agentID string, envelope *grpcapi.SchedulerRelayEnvelope, msg *grpcapi.ExecutorMessage) error {
	if msg == nil {
		return nil
	}
	if msg.GetHello() != nil {
		hello := msg.GetHello()
		relaySessionID := strings.TrimSpace(envelope.GetRelaySessionId())
		if relaySessionID == "" {
			return status.Error(codes.InvalidArgument, "relay_session_id required")
		}
		if strings.TrimSpace(hello.GetExecutorId()) == "" {
			return status.Error(codes.InvalidArgument, "executor_id required")
		}
		var executor *model.SchedulerExecutor
		var err error
		serviceInstance := hello.GetMetadata()[schedulerServiceInstanceMetadataKey] == "true"
		if serviceInstance {
			executor, err = s.service.RegisterServiceInstanceExecutor(
				hello.GetExecutorId(),
				hello.GetGroup(),
				model.Slot(hello.GetLiveSlot()),
				hello.GetMetadata(),
				agentID,
				envelope.GetRoutingKey(),
			)
		} else {
			executor, err = s.service.AuthenticateExecutor(
				hello.GetExecutorId(),
				hello.GetToken(),
				hello.GetGroup(),
				model.Slot(hello.GetLiveSlot()),
				hello.GetMetadata(),
				agentID,
				envelope.GetRoutingKey(),
			)
		}
		if err != nil {
			return err
		}
		executorMeta := map[string]string{}
		if executor.InstanceMeta != nil {
			executorMeta = executor.InstanceMeta.Get()
		}
		s.hub.registerRelay(hello.GetExecutorId(), executor.Group, executor.LiveSlot, agentID, relaySessionID, envelope.GetRoutingKey(), serviceInstance, executorMeta["service_id"], s.agentHub)
		ack := &grpcapi.SchedulerMessage{Payload: &grpcapi.SchedulerMessage_Ack{Ack: &grpcapi.SchedulerAck{Message: "connected"}}}
		session := s.hub.getRelay(relaySessionID, agentID)
		if session != nil {
			_ = session.send(ack)
		}
		return nil
	}
	session := s.hub.getRelay(envelope.GetRelaySessionId(), agentID)
	if session == nil {
		s.logger.Infof("dropping relay message for unknown session: agentId=%s relaySessionId=%s", agentID, envelope.GetRelaySessionId())
		return nil
	}
	s.handleExecutorInbound(session.executorID, msg)
	return nil
}

func (s *SchedulerServer) handleExecutorInbound(executorID string, msg *grpcapi.ExecutorMessage) {
	switch {
	case msg.GetHeartbeat() != nil:
		_ = s.service.HeartbeatExecutor(executorID)
	case msg.GetLeaseRenew() != nil:
		runID, parseErr := uuid.Parse(msg.GetLeaseRenew().GetRunId())
		if parseErr == nil {
			_ = s.service.RenewRunLease(runID, executorID)
		}
	case msg.GetRunUpdate() != nil:
		runID, parseErr := uuid.Parse(msg.GetRunUpdate().GetRunId())
		if parseErr != nil {
			return
		}
		update := msg.GetRunUpdate()
		if update.GetRunning() {
			_ = s.service.MarkRunRunning(runID, executorID)
			return
		}
		_ = s.service.CompleteRun(runID, executorID, update.GetSuccess(), update.GetRetryable(), update.GetErrorMessage())
	}
}

func (s *executorSession) send(message *grpcapi.SchedulerMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return status.Error(codes.Unavailable, "executor offline")
	}
	if s.channelType == schedulerChannelRelay {
		if s.agentHub == nil {
			return status.Error(codes.Unavailable, "relay unavailable")
		}
		payloadBytes, err := proto.Marshal(message)
		if err != nil {
			return err
		}
		return s.agentHub.DispatchSchedulerEnvelope(s.agentID, &grpcapi.SchedulerRelayEnvelope{
			RelaySessionId: s.relaySessionID,
			ExecutorId:     s.executorID,
			RoutingKey:     s.routingKey,
			AgentId:        s.agentID,
			Direction:      grpcapi.SchedulerRelayDirection_SCHEDULER_RELAY_DIRECTION_DOWNSTREAM,
			PayloadType:    grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SCHEDULER_MESSAGE,
			PayloadBytes:   payloadBytes,
		})
	}
	s.sendCh <- message
	return nil
}

func (s *executorSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.channelType == schedulerChannelDirect {
		close(s.sendCh)
	}
	s.closed = true
}
