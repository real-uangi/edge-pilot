package controlplane

import (
	schedulerapp "edge-pilot/internal/scheduler/application"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type schedulerSessionHub struct {
	mu       sync.RWMutex
	sessions map[string]*executorSession
}

type executorSession struct {
	mu         sync.Mutex
	executorID string
	group      string
	liveSlot   model.Slot
	sendCh     chan *grpcapi.SchedulerMessage
	closed     bool
}

func NewSchedulerSessionHub() *schedulerSessionHub {
	return &schedulerSessionHub{sessions: make(map[string]*executorSession)}
}

func (h *schedulerSessionHub) register(executorID string, group string, liveSlot model.Slot) *executorSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.sessions[executorID]; ok {
		existing.close()
	}
	session := &executorSession{
		executorID: executorID,
		group:      group,
		liveSlot:   liveSlot,
		sendCh:     make(chan *grpcapi.SchedulerMessage, 16),
	}
	h.sessions[executorID] = session
	return session
}

func (h *schedulerSessionHub) unregister(executorID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.sessions[executorID]; ok {
		s.close()
		delete(h.sessions, executorID)
	}
}

func (h *schedulerSessionHub) ListOnlineExecutors(group string) []schedulerapp.OnlineExecutor {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]schedulerapp.OnlineExecutor, 0)
	for _, session := range h.sessions {
		if group != "" && session.group != group {
			continue
		}
		out = append(out, schedulerapp.OnlineExecutor{
			ExecutorID: session.executorID,
			Group:      session.group,
			LiveSlot:   session.liveSlot,
		})
	}
	return out
}

func (h *schedulerSessionHub) DispatchRun(executorID string, run *model.SchedulerJobRun) error {
	h.mu.RLock()
	session, ok := h.sessions[executorID]
	h.mu.RUnlock()
	if !ok {
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
				TaskType:       run.TaskType,
				IdempotencyKey: run.IdempotencyKey,
				Attempt:        int32(run.Attempt),
				PayloadJson:    string(payloadBytes),
			},
		},
	})
}

type SchedulerServer struct {
	grpcapi.UnimplementedSchedulerControlServer

	hub     *schedulerSessionHub
	service *schedulerapp.Service
	logger  *log.StdLogger
}

func NewSchedulerServer(hub *schedulerSessionHub, service *schedulerapp.Service) *SchedulerServer {
	return &SchedulerServer{
		hub:     hub,
		service: service,
		logger:  log.NewStdLogger("grpc.scheduler"),
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
	)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	session := s.hub.register(executorID, executor.Group, executor.LiveSlot)
	defer s.hub.unregister(executorID)

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
				continue
			}
			update := msg.GetRunUpdate()
			if update.GetRunning() {
				_ = s.service.MarkRunRunning(runID, executorID)
				continue
			}
			_ = s.service.CompleteRun(runID, executorID, update.GetSuccess(), update.GetRetryable(), update.GetErrorMessage())
		}
	}
}

func (s *executorSession) send(message *grpcapi.SchedulerMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return status.Error(codes.Unavailable, "executor offline")
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
	close(s.sendCh)
	s.closed = true
}
