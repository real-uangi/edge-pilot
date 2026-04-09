package agent

import (
	"context"
	agentapp "edge-pilot/internal/agent/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"sync"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	cfg      *config.AgentRuntimeConfig
	executor *agentapp.Executor
	state    *agentapp.RuntimeState
}

func NewClient(cfg *config.AgentRuntimeConfig, executor *agentapp.Executor, state *agentapp.RuntimeState) *Client {
	return &Client{
		cfg:      cfg,
		executor: executor,
		state:    state,
	}
}

func startClient(lc fx.Lifecycle, client *Client) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go client.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func (c *Client) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := c.connectOnce(ctx); err != nil {
			time.Sleep(3 * time.Second)
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) error {
	conn, err := grpc.DialContext(
		ctx,
		c.cfg.ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(grpcapi.JSONCodecName)),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := grpcapi.NewAgentControlClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return err
	}
	outbound := make(chan *grpcapi.AgentMessage, 32)
	var sendMu sync.Mutex
	go func() {
		for msg := range outbound {
			sendMu.Lock()
			_ = stream.Send(msg)
			sendMu.Unlock()
		}
	}()

	outbound <- &grpcapi.AgentMessage{
		Kind: "hello",
		Hello: &grpcapi.HelloMessage{
			AgentID:      c.cfg.AgentID,
			Token:        c.cfg.AgentToken,
			Version:      c.cfg.AgentVersion,
			Hostname:     c.cfg.Hostname,
			Capabilities: []string{"docker", "haproxy", "http_probe"},
		},
	}

	heartbeatTicker := time.NewTicker(5 * time.Second)
	statsTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()
	defer statsTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatTicker.C:
				outbound <- &grpcapi.AgentMessage{
					Kind: "heartbeat",
					Heartbeat: &grpcapi.HeartbeatMessage{
						AgentID:        c.cfg.AgentID,
						RunningTaskIDs: c.state.RunningTaskIDs(),
					},
				}
			case <-statsTicker.C:
				stats, err := c.executor.CollectStats(ctx)
				if err == nil && len(stats) > 0 {
					outbound <- &grpcapi.AgentMessage{
						Kind: "stats",
						Stats: &grpcapi.StatsReport{
							AgentID:  c.cfg.AgentID,
							Services: stats,
						},
					}
				}
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			close(outbound)
			return err
		}
		if msg.Task != nil {
			task := msg.Task
			go c.handleTask(ctx, task, outbound)
		}
	}
}

func (c *Client) handleTask(ctx context.Context, task *grpcapi.TaskCommand, outbound chan<- *grpcapi.AgentMessage) {
	c.state.Start(task.TaskID)
	defer c.state.Done(task.TaskID)
	err := c.executor.Execute(ctx, task, func(update *grpcapi.TaskUpdate) error {
		outbound <- &grpcapi.AgentMessage{
			Kind:       "task_update",
			TaskUpdate: update,
		}
		return nil
	})
	if err != nil {
		outbound <- &grpcapi.AgentMessage{
			Kind: "task_update",
			TaskUpdate: &grpcapi.TaskUpdate{
				TaskID:       task.TaskID,
				Status:       "failed",
				Step:         "execution_failed",
				ErrorMessage: err.Error(),
			},
		}
	}
}
