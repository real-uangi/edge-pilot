package agent

import (
	"context"
	agentapp "edge-pilot/internal/agent/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"sync"
	"time"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	cfg      *config.AgentRuntimeConfig
	executor *agentapp.Executor
	proxy    agentapp.ProxyRuntime
	state    *agentapp.RuntimeState
	logger   *log.StdLogger
}

func NewClient(cfg *config.AgentRuntimeConfig, executor *agentapp.Executor, proxy agentapp.ProxyRuntime, state *agentapp.RuntimeState) *Client {
	return &Client{
		cfg:      cfg,
		executor: executor,
		proxy:    proxy,
		state:    state,
		logger:   log.NewStdLogger("agent.grpc-client"),
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
			c.logger.Errorf(err, "grpc client disconnected, retrying in 3s: agentId=%s addr=%s", c.cfg.AgentID, c.cfg.ControlPlaneAddr)
			time.Sleep(3 * time.Second)
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) error {
	c.logger.Infof("connecting to control-plane: agentId=%s addr=%s", c.cfg.AgentID, c.cfg.ControlPlaneAddr)
	conn, err := grpc.DialContext(
		ctx,
		c.cfg.ControlPlaneAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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
			if err := stream.Send(msg); err != nil {
				c.logger.Errorf(err, "failed to send grpc message: agentId=%s addr=%s", c.cfg.AgentID, c.cfg.ControlPlaneAddr)
			}
			sendMu.Unlock()
		}
	}()

	outbound <- &grpcapi.AgentMessage{
		Payload: &grpcapi.AgentMessage_Hello{
			Hello: &grpcapi.HelloMessage{
				AgentId:      c.cfg.AgentID,
				Token:        c.cfg.AgentToken,
				Version:      c.cfg.AgentVersion,
				Hostname:     c.cfg.Hostname,
				Capabilities: []string{"docker", "haproxy_runtime", "haproxy_dataplane", "http_probe"},
			},
		},
	}
	c.logger.Infof("grpc hello sent: agentId=%s version=%s", c.cfg.AgentID, c.cfg.AgentVersion)

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
					Payload: &grpcapi.AgentMessage_Heartbeat{
						Heartbeat: &grpcapi.HeartbeatMessage{
							AgentId:        c.cfg.AgentID,
							RunningTaskIds: c.state.RunningTaskIDs(),
						},
					},
				}
			case <-statsTicker.C:
				stats, err := c.executor.CollectStats(ctx)
				if err != nil {
					c.logger.Errorf(err, "failed to collect stats: agentId=%s", c.cfg.AgentID)
					continue
				}
				if len(stats) > 0 {
					outbound <- &grpcapi.AgentMessage{
						Payload: &grpcapi.AgentMessage_Stats{
							Stats: &grpcapi.StatsReport{
								AgentId:  c.cfg.AgentID,
								Services: stats,
							},
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
			c.logger.Errorf(err, "grpc stream receive failed: agentId=%s addr=%s", c.cfg.AgentID, c.cfg.ControlPlaneAddr)
			return err
		}
		if msg.GetAck() != nil {
			c.logger.Infof("grpc connected: agentId=%s message=%s", c.cfg.AgentID, msg.GetAck().GetMessage())
			continue
		}
		if msg.GetTask() != nil {
			task := msg.GetTask()
			c.logger.Infof("received task: agentId=%s taskId=%s type=%s serviceKey=%s", c.cfg.AgentID, task.GetTaskId(), task.GetType().String(), task.GetServiceKey())
			go c.handleTask(ctx, task, outbound)
			continue
		}
		if msg.GetProxyConfig() != nil {
			if err := c.proxy.ApplySnapshot(ctx, msg.GetProxyConfig()); err != nil {
				c.logger.Errorf(err, "failed to apply proxy snapshot: agentId=%s services=%d", c.cfg.AgentID, len(msg.GetProxyConfig().GetServices()))
				continue
			}
			c.logger.Infof("proxy snapshot applied: agentId=%s services=%d", c.cfg.AgentID, len(msg.GetProxyConfig().GetServices()))
		}
	}
}

func (c *Client) handleTask(ctx context.Context, task *grpcapi.TaskCommand, outbound chan<- *grpcapi.AgentMessage) {
	if !c.state.TryStart(task.GetTaskId()) {
		c.logger.Infof("skip duplicated running task: agentId=%s taskId=%s", c.cfg.AgentID, task.GetTaskId())
		return
	}
	defer c.state.Done(task.GetTaskId())
	c.logger.Infof("start task execution: agentId=%s taskId=%s type=%s serviceKey=%s", c.cfg.AgentID, task.GetTaskId(), task.GetType().String(), task.GetServiceKey())
	err := c.executor.Execute(ctx, task, func(update *grpcapi.TaskUpdate) error {
		outbound <- &grpcapi.AgentMessage{
			Payload: &grpcapi.AgentMessage_TaskUpdate{
				TaskUpdate: update,
			},
		}
		return nil
	})
	if err != nil {
		c.logger.Errorf(err, "task execution failed: agentId=%s taskId=%s type=%s", c.cfg.AgentID, task.GetTaskId(), task.GetType().String())
		step := "execution_failed"
		if execErr, ok := err.(*agentapp.TaskExecutionError); ok && execErr.Step != "" {
			step = execErr.Step
		}
		outbound <- &grpcapi.AgentMessage{
			Payload: &grpcapi.AgentMessage_TaskUpdate{
				TaskUpdate: &grpcapi.TaskUpdate{
					TaskId:       task.GetTaskId(),
					Status:       grpcapi.TaskStatus_TASK_STATUS_FAILED,
					Step:         step,
					ErrorMessage: err.Error(),
				},
			},
		}
		return
	}
	c.logger.Infof("task execution finished: agentId=%s taskId=%s type=%s", c.cfg.AgentID, task.GetTaskId(), task.GetType().String())
}
