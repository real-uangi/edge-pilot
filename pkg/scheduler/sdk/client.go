package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Handler func(ctx context.Context, run RunContext) error

type RunContext struct {
	JobRunID       string
	HandlerKey     string
	Payload        map[string]any
	IdempotencyKey string
	Attempt        int

	ServiceID   string
	ReleaseID   string
	Slot        string
	AgentID     string
	ExecutorID  string
	ServiceKey  string
	BackendName string
	ServerName  string
}

type RunResult struct {
	Success   bool
	Retryable bool
	Err       error
}

type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	if e == nil || e.Err == nil {
		return "retryable error"
	}
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type ExecutorClientOptions struct {
	Addr       string
	ExecutorID string
	Token      string
	Group      string
	InstanceID string
	LiveSlot   grpcapi.Slot
	Metadata   map[string]string
	RelayToken string

	// Deprecated: keep for backward compatibility with old SDK options.
	Mode string
	// Deprecated: use Addr instead.
	RelayAddr string
}

type ExecutorClient struct {
	opts     ExecutorClientOptions
	handlers map[string]Handler
	mu       sync.RWMutex
}

func NewExecutorClient(opts ExecutorClientOptions) *ExecutorClient {
	addr := strings.TrimSpace(opts.Addr)
	if addr == "" {
		addr = strings.TrimSpace(opts.RelayAddr)
	}
	meta := map[string]string{}
	for k, v := range opts.Metadata {
		meta[k] = v
	}
	if opts.InstanceID != "" {
		meta["instanceId"] = opts.InstanceID
	}
	if opts.RelayToken != "" {
		meta["relay_token"] = opts.RelayToken
	}
	return &ExecutorClient{
		opts: ExecutorClientOptions{
			Addr:       addr,
			ExecutorID: opts.ExecutorID,
			Token:      opts.Token,
			Group:      opts.Group,
			InstanceID: opts.InstanceID,
			LiveSlot:   opts.LiveSlot,
			Metadata:   meta,
			RelayToken: opts.RelayToken,
			Mode:       opts.Mode,
			RelayAddr:  opts.RelayAddr,
		},
		handlers: make(map[string]Handler),
	}
}

func NewExecutorClientFromEnv() *ExecutorClient {
	return &ExecutorClient{
		opts: ExecutorClientOptions{
			Addr:       envAddr(),
			ExecutorID: envOrDefault("EP_EXECUTOR_ID", ""),
			Token:      "",
			Group:      envOrDefault("EP_SCHEDULER_EXECUTOR_GROUP", ""),
			LiveSlot:   envLiveSlot(),
			Metadata:   envMetadata(),
		},
		handlers: make(map[string]Handler),
	}
}

func envAddr() string {
	addr := strings.TrimSpace(os.Getenv("EP_SCHEDULER_SDK_ADDR"))
	if addr == "" {
		addr = "127.0.0.1"
	}
	portStr := strings.TrimSpace(os.Getenv("EP_SCHEDULER_SDK_PORT"))
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return addr
	}
	return addr + ":" + portStr
}

func envLiveSlot() grpcapi.Slot {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("EP_SLOT"))) {
	case "blue":
		return grpcapi.Slot_SLOT_BLUE
	case "green":
		return grpcapi.Slot_SLOT_GREEN
	default:
		return grpcapi.Slot_SLOT_UNSPECIFIED
	}
}

func envMetadata() map[string]string {
	meta := map[string]string{}
	for _, key := range []string{"EP_SERVICE_ID", "EP_SERVICE_KEY", "EP_RELEASE_ID", "EP_AGENT_ID", "EP_BACKEND_NAME", "EP_SERVER_NAME"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			meta[strings.ToLower(strings.TrimPrefix(key, "EP_"))] = v
		}
	}
	return meta
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func (c *ExecutorClient) RegisterHandler(handlerKey string, handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[handlerKey] = handler
}

func (c *ExecutorClient) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := c.connectOnce(ctx); err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (c *ExecutorClient) connectOnce(ctx context.Context) error {
	addr := strings.TrimSpace(c.opts.Addr)
	if addr == "" {
		return errors.New("scheduler target addr is required")
	}
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := grpcapi.NewSchedulerControlClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return err
	}

	outbound := make(chan *grpcapi.ExecutorMessage, 32)
	runningMu := sync.Mutex{}
	running := make(map[string]struct{})

	go func() {
		for msg := range outbound {
			_ = stream.Send(msg)
		}
	}()

	outbound <- &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_Hello{Hello: &grpcapi.ExecutorHello{
		ExecutorId: c.opts.ExecutorID,
		Token:      c.opts.Token,
		Group:      c.opts.Group,
		LiveSlot:   c.opts.LiveSlot,
		Metadata:   c.opts.Metadata,
	}}}

	heartbeatTicker := time.NewTicker(5 * time.Second)
	defer heartbeatTicker.Stop()
	leaseTicker := time.NewTicker(10 * time.Second)
	defer leaseTicker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatTicker.C:
				runningMu.Lock()
				ids := make([]string, 0, len(running))
				for id := range running {
					ids = append(ids, id)
				}
				runningMu.Unlock()
				outbound <- &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_Heartbeat{Heartbeat: &grpcapi.ExecutorHeartbeat{
					ExecutorId:    c.opts.ExecutorID,
					RunningRunIds: ids,
				}}}
			case <-leaseTicker.C:
				runningMu.Lock()
				ids := make([]string, 0, len(running))
				for id := range running {
					ids = append(ids, id)
				}
				runningMu.Unlock()
				for _, runID := range ids {
					outbound <- &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_LeaseRenew{LeaseRenew: &grpcapi.SchedulerRunLeaseRenew{RunId: runID}}}
				}
			}
		}
	}()

	for {
		msg, recvErr := stream.Recv()
		if recvErr != nil {
			close(outbound)
			return recvErr
		}
		if msg.GetAck() != nil {
			continue
		}
		runCmd := msg.GetRun()
		if runCmd == nil {
			continue
		}
		go func(command *grpcapi.SchedulerRunCommand) {
			runID := command.GetRunId()
			runningMu.Lock()
			running[runID] = struct{}{}
			runningMu.Unlock()
			defer func() {
				runningMu.Lock()
				delete(running, runID)
				runningMu.Unlock()
			}()

			outbound <- &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_RunUpdate{RunUpdate: &grpcapi.SchedulerRunUpdate{
				RunId:   runID,
				Running: true,
			}}}

			ctxRun := RunContext{
				JobRunID:       runID,
				HandlerKey:     command.GetHandlerKey(),
				IdempotencyKey: command.GetIdempotencyKey(),
				Attempt:        int(command.GetAttempt()),
				Payload:        map[string]any{},
				ServiceID:      envOrDefault("EP_SERVICE_ID", ""),
				ReleaseID:      envOrDefault("EP_RELEASE_ID", ""),
				Slot:           envOrDefault("EP_SLOT", ""),
				AgentID:        envOrDefault("EP_AGENT_ID", ""),
				ExecutorID:     envOrDefault("EP_EXECUTOR_ID", ""),
				ServiceKey:     envOrDefault("EP_SERVICE_KEY", ""),
				BackendName:    envOrDefault("EP_BACKEND_NAME", ""),
				ServerName:     envOrDefault("EP_SERVER_NAME", ""),
			}
			if raw := command.GetPayloadJson(); raw != "" {
				_ = json.Unmarshal([]byte(raw), &ctxRun.Payload)
			}
			handler := c.getHandler(ctxRun.HandlerKey)
			if handler == nil {
				outbound <- &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_RunUpdate{RunUpdate: &grpcapi.SchedulerRunUpdate{
					RunId:        runID,
					Success:      false,
					Retryable:    false,
					ErrorMessage: "no handler registered for handlerKey",
				}}}
				return
			}
			err := handler(ctx, ctxRun)
			if err == nil {
				outbound <- &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_RunUpdate{RunUpdate: &grpcapi.SchedulerRunUpdate{
					RunId:   runID,
					Success: true,
				}}}
				return
			}
			retryable := false
			var retryableErr *RetryableError
			if errors.As(err, &retryableErr) {
				retryable = true
			}
			outbound <- &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_RunUpdate{RunUpdate: &grpcapi.SchedulerRunUpdate{
				RunId:        runID,
				Success:      false,
				Retryable:    retryable,
				ErrorMessage: err.Error(),
			}}}
		}(runCmd)
	}
}

func (c *ExecutorClient) getHandler(handlerKey string) Handler {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.handlers[handlerKey]
}
