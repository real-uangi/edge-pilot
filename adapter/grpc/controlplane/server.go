package controlplane

import (
	"context"
	agentapp "edge-pilot/internal/agent/application"
	observabilityapp "edge-pilot/internal/observability/application"
	releaseapp "edge-pilot/internal/release/application"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"fmt"
	"net"
	"os"
	"sync"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type sessionHub struct {
	mu       sync.RWMutex
	sessions map[string]*agentSession
}

type agentSession struct {
	agentID string
	sendCh  chan *grpcapi.ControlMessage
}

func NewSessionHub() *sessionHub {
	return &sessionHub{
		sessions: make(map[string]*agentSession),
	}
}

func (h *sessionHub) register(agentID string) *agentSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	session := &agentSession{
		agentID: agentID,
		sendCh:  make(chan *grpcapi.ControlMessage, 16),
	}
	h.sessions[agentID] = session
	return session
}

func (h *sessionHub) unregister(agentID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if session, ok := h.sessions[agentID]; ok {
		close(session.sendCh)
		delete(h.sessions, agentID)
	}
}

func (h *sessionHub) DispatchTask(agentID string, task *model.Task) error {
	h.mu.RLock()
	session, ok := h.sessions[agentID]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("agent %s is offline", agentID)
	}
	payload := getPayload(task)
	_ = payload
	session.sendCh <- &grpcapi.ControlMessage{
		Payload: &grpcapi.ControlMessage_Task{
			Task: taskToProto(task),
		},
	}
	return nil
}

type Server struct {
	grpcapi.UnimplementedAgentControlServer
	hub           *sessionHub
	agents        *agentapp.RegistryService
	releases      *releaseapp.Service
	observability *observabilityapp.Service
}

func NewServer(hub *sessionHub, agents *agentapp.RegistryService, releases *releaseapp.Service, observability *observabilityapp.Service) *Server {
	return &Server{
		hub:           hub,
		agents:        agents,
		releases:      releases,
		observability: observability,
	}
}

func (s *Server) Connect(stream grpcapi.AgentControl_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if first.GetHello() == nil {
		return status.Error(codes.InvalidArgument, "hello required")
	}
	hello := first.GetHello()
	if !s.agents.Authenticate(hello.GetAgentId(), hello.GetToken()) {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	if err := s.agents.MarkConnected(hello.GetAgentId(), hello.GetHostname(), hello.GetVersion(), hello.GetCapabilities()); err != nil {
		return err
	}
	session := s.hub.register(hello.GetAgentId())
	defer func() {
		s.hub.unregister(hello.GetAgentId())
		_ = s.agents.MarkDisconnected(hello.GetAgentId(), "stream disconnected")
	}()

	sendErrCh := make(chan error, 1)
	go func() {
		for msg := range session.sendCh {
			if err := stream.Send(msg); err != nil {
				sendErrCh <- err
				return
			}
		}
	}()

	if err := stream.Send(&grpcapi.ControlMessage{
		Payload: &grpcapi.ControlMessage_Ack{
			Ack: &grpcapi.AckMessage{Message: "connected"},
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case err := <-sendErrCh:
			return err
		default:
		}
		message, err := stream.Recv()
		if err != nil {
			return err
		}
		switch {
		case message.GetHeartbeat() != nil:
			if err := s.agents.Heartbeat(message.GetHeartbeat().GetAgentId()); err != nil {
				return err
			}
		case message.GetTaskUpdate() != nil:
			if err := s.releases.HandleTaskUpdate(hello.GetAgentId(), message.GetTaskUpdate()); err != nil {
				return err
			}
		case message.GetStats() != nil:
			if err := s.observability.RecordStats(message.GetStats()); err != nil {
				return err
			}
		}
	}
}

func startGRPCServer(lc fx.Lifecycle, server *Server) {
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "9090"
	}
	grpcServer := grpc.NewServer()
	grpcapi.RegisterAgentControlServer(grpcServer, server)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", ":"+port)
			if err != nil {
				return err
			}
			go func() {
				_ = grpcServer.Serve(lis)
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			done := make(chan struct{})
			go func() {
				grpcServer.GracefulStop()
				close(done)
			}()
			select {
			case <-ctx.Done():
				grpcServer.Stop()
				return ctx.Err()
			case <-done:
				return nil
			}
		},
	})
}

func getPayload(task *model.Task) model.TaskPayload {
	if task.Payload == nil {
		return model.TaskPayload{}
	}
	return task.Payload.Get()
}
