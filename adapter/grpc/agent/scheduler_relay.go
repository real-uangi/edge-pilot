package agent

import (
	"context"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type schedulerRelayBridge struct {
	cfg    *config.AgentRuntimeConfig
	logger *log.StdLogger

	mu       sync.RWMutex
	sessions map[string]*relayExecutorSession
	sender   chan<- *grpcapi.AgentMessage

	server *grpc.Server
	lis    net.Listener
}

type relayExecutorSession struct {
	id         string
	executorID string
	routingKey string
	sendCh     chan *grpcapi.SchedulerMessage
	done       chan struct{}
	doneOnce   sync.Once
}

func (s *relayExecutorSession) close() {
	if s == nil {
		return
	}
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

func (s *relayExecutorSession) trySend(message *grpcapi.SchedulerMessage) bool {
	if s == nil || message == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
	}
	select {
	case <-s.done:
		return false
	case s.sendCh <- message:
		return true
	default:
		return false
	}
}

type localSchedulerRelayServer struct {
	grpcapi.UnimplementedSchedulerControlServer
	bridge *schedulerRelayBridge
}

func newSchedulerRelayBridge(cfg *config.AgentRuntimeConfig) *schedulerRelayBridge {
	return &schedulerRelayBridge{
		cfg:      cfg,
		logger:   log.NewStdLogger("agent.scheduler-relay"),
		sessions: make(map[string]*relayExecutorSession),
	}
}

func (b *schedulerRelayBridge) Start() error {
	if b.cfg == nil || b.cfg.SchedulerRelayListenAddr == "" {
		return nil
	}
	lis, err := net.Listen("tcp", b.cfg.SchedulerRelayListenAddr)
	if err != nil {
		return err
	}
	server := grpc.NewServer()
	grpcapi.RegisterSchedulerControlServer(server, &localSchedulerRelayServer{bridge: b})
	b.mu.Lock()
	b.lis = lis
	b.server = server
	b.mu.Unlock()
	go func() {
		if serveErr := server.Serve(lis); serveErr != nil {
			b.logger.Errorf(serveErr, "scheduler relay server stopped")
		}
	}()
	b.logger.Infof("scheduler relay listening: addr=%s", lis.Addr().String())
	return nil
}

func (b *schedulerRelayBridge) Stop(ctx context.Context) {
	b.mu.Lock()
	server := b.server
	lis := b.lis
	b.server = nil
	b.lis = nil
	b.mu.Unlock()
	if server != nil {
		done := make(chan struct{})
		go func() {
			server.GracefulStop()
			close(done)
		}()
		select {
		case <-ctx.Done():
			server.Stop()
		case <-done:
		}
	}
	if lis != nil {
		_ = lis.Close()
	}
}

func (b *schedulerRelayBridge) SetOutbound(outbound chan<- *grpcapi.AgentMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sender = outbound
}

func (b *schedulerRelayBridge) clearOutbound() {
	b.SetOutbound(nil)
}

func (b *schedulerRelayBridge) sendUpstream(msg *grpcapi.AgentMessage) error {
	b.mu.RLock()
	sender := b.sender
	b.mu.RUnlock()
	if sender == nil {
		return status.Error(codes.Unavailable, "control-plane stream not ready")
	}
	sender <- msg
	return nil
}

func (b *schedulerRelayBridge) registerSession(session *relayExecutorSession) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sessions[session.id] = session
}

func (b *schedulerRelayBridge) unregisterSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if session, ok := b.sessions[sessionID]; ok {
		session.close()
		delete(b.sessions, sessionID)
	}
}

func (b *schedulerRelayBridge) getSession(sessionID string) *relayExecutorSession {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sessions[sessionID]
}

func (b *schedulerRelayBridge) HandleControlEnvelope(envelope *grpcapi.SchedulerRelayEnvelope) {
	if envelope == nil {
		return
	}
	sessionID := envelope.GetRelaySessionId()
	if sessionID == "" {
		return
	}
	session := b.getSession(sessionID)
	if session == nil {
		return
	}
	switch envelope.GetPayloadType() {
	case grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SCHEDULER_MESSAGE:
		var message grpcapi.SchedulerMessage
		if err := proto.Unmarshal(envelope.GetPayloadBytes(), &message); err != nil {
			return
		}
		_ = session.trySend(&message)
	case grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SESSION_CLOSED,
		grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SESSION_ERROR:
		b.unregisterSession(sessionID)
	}
}

func (s *localSchedulerRelayServer) Connect(stream grpcapi.SchedulerControl_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil {
		return status.Error(codes.InvalidArgument, "hello required")
	}
	if token := s.bridge.cfg.SchedulerRelaySharedToken; token != "" {
		if hello.GetMetadata()["relay_token"] != token {
			return status.Error(codes.Unauthenticated, "invalid relay token")
		}
	}
	session := &relayExecutorSession{
		id:         uuid.NewString(),
		executorID: hello.GetExecutorId(),
		routingKey: hello.GetRoutingKey(),
		sendCh:     make(chan *grpcapi.SchedulerMessage, 16),
		done:       make(chan struct{}),
	}
	if session.routingKey == "" {
		session.routingKey = session.executorID
	}
	s.bridge.registerSession(session)
	defer s.bridge.unregisterSession(session.id)

	sendErrCh := make(chan error, 1)
	go func() {
		for {
			select {
			case <-session.done:
				return
			case msg := <-session.sendCh:
				if sendErr := stream.Send(msg); sendErr != nil {
					sendErrCh <- sendErr
					return
				}
			}
		}
	}()

	helloBytes, marshalErr := proto.Marshal(first)
	if marshalErr != nil {
		return marshalErr
	}
	if err := s.bridge.sendUpstream(&grpcapi.AgentMessage{Payload: &grpcapi.AgentMessage_SchedulerEnvelope{SchedulerEnvelope: &grpcapi.SchedulerRelayEnvelope{
		RelaySessionId: session.id,
		ExecutorId:     session.executorID,
		RoutingKey:     session.routingKey,
		AgentId:        s.bridge.cfg.AgentID,
		Direction:      grpcapi.SchedulerRelayDirection_SCHEDULER_RELAY_DIRECTION_UPSTREAM,
		PayloadType:    grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_EXECUTOR_MESSAGE,
		PayloadBytes:   helloBytes,
	}}}); err != nil {
		return err
	}

	for {
		select {
		case sendErr := <-sendErrCh:
			return sendErr
		default:
		}
		message, recvErr := stream.Recv()
		if recvErr != nil {
			_ = s.bridge.sendUpstream(&grpcapi.AgentMessage{Payload: &grpcapi.AgentMessage_SchedulerEnvelope{SchedulerEnvelope: &grpcapi.SchedulerRelayEnvelope{
				RelaySessionId: session.id,
				ExecutorId:     session.executorID,
				RoutingKey:     session.routingKey,
				AgentId:        s.bridge.cfg.AgentID,
				Direction:      grpcapi.SchedulerRelayDirection_SCHEDULER_RELAY_DIRECTION_CONTROL,
				PayloadType:    grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SESSION_CLOSED,
			}}})
			return recvErr
		}
		payloadBytes, marshalErr := proto.Marshal(message)
		if marshalErr != nil {
			continue
		}
		_ = s.bridge.sendUpstream(&grpcapi.AgentMessage{Payload: &grpcapi.AgentMessage_SchedulerEnvelope{SchedulerEnvelope: &grpcapi.SchedulerRelayEnvelope{
			RelaySessionId: session.id,
			ExecutorId:     session.executorID,
			RoutingKey:     session.routingKey,
			AgentId:        s.bridge.cfg.AgentID,
			Direction:      grpcapi.SchedulerRelayDirection_SCHEDULER_RELAY_DIRECTION_UPSTREAM,
			PayloadType:    grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_EXECUTOR_MESSAGE,
			PayloadBytes:   payloadBytes,
		}}})
	}
}
