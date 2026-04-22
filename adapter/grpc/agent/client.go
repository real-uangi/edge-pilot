package agent

import (
	"context"
	"edge-pilot/internal/agent/application/taskexec"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/perf"
	"errors"
	"sync"
	"time"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	cfg       *config.AgentRuntimeConfig
	executor  *taskexec.Executor
	proxy     agentdomain.ProxyRuntime
	state     *taskexec.RuntimeState
	collector perf.Collector
	logger    *log.StdLogger
}

func NewClient(cfg *config.AgentRuntimeConfig, executor *taskexec.Executor, proxy agentdomain.ProxyRuntime, state *taskexec.RuntimeState, collector perf.Collector) *Client {
	return &Client{
		cfg:       cfg,
		executor:  executor,
		proxy:     proxy,
		state:     state,
		collector: collector,
		logger:    log.NewStdLogger("agent.grpc-client"),
	}
}

func startClient(lc fx.Lifecycle, client *Client) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			client.logger.Infof("starting grpc client after proxy stack preparation: agentId=%s addr=%s", client.cfg.AgentID, client.cfg.ControlPlaneAddr)
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
			c.logger.Errorf(err, "grpc client retrying connection in 3s: agentId=%s addr=%s", c.cfg.AgentID, c.cfg.ControlPlaneAddr)
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
		c.logger.Errorf(err, "failed to open grpc stream: agentId=%s addr=%s", c.cfg.AgentID, c.cfg.ControlPlaneAddr)
		return err
	}
	c.logger.Infof("opening grpc stream: agentId=%s addr=%s", c.cfg.AgentID, c.cfg.ControlPlaneAddr)
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
				ReportedIp:   c.cfg.ReportedIP,
			},
		},
	}
	c.logger.Infof("waiting for grpc ack: agentId=%s version=%s", c.cfg.AgentID, c.cfg.AgentVersion)

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
				c.logger.Infof("sending heartbeat: agentId=%s runningTasks=%d", c.cfg.AgentID, len(c.state.RunningTaskIDs()))
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
					if errors.Is(err, agentdomain.ErrProxyNotReady) {
						continue
					}
					c.logger.Errorf(err, "failed to collect stats: agentId=%s", c.cfg.AgentID)
					continue
				}
				var selfPerformance *grpcapi.PerformanceSnapshot
				selfSnapshot, selfErr := c.collector.Collect(ctx)
				if selfErr != nil {
					c.logger.Errorf(selfErr, "failed to collect self performance: agentId=%s", c.cfg.AgentID)
				} else if selfSnapshot != nil {
					selfPerformance = &grpcapi.PerformanceSnapshot{
						CpuPercent:        selfSnapshot.CPUPercent,
						MemoryUsedBytes:   selfSnapshot.MemoryUsedBytes,
						MemoryLimitBytes:  selfSnapshot.MemoryLimitBytes,
						Source:            selfSnapshot.Source,
						CollectedAtUnixMs: selfSnapshot.CollectedAt.UnixMilli(),
					}
				}
				if len(stats) > 0 || selfPerformance != nil {
					c.logger.Infof("sending stats report: agentId=%s entries=%d", c.cfg.AgentID, len(stats))
					outbound <- &grpcapi.AgentMessage{
						Payload: &grpcapi.AgentMessage_Stats{
							Stats: &grpcapi.StatsReport{
								AgentId:         c.cfg.AgentID,
								Services:        stats,
								SelfPerformance: selfPerformance,
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
			c.logger.Infof("connected to control-plane: agentId=%s message=%s", c.cfg.AgentID, msg.GetAck().GetMessage())
			continue
		}
		if msg.GetTask() != nil {
			task := msg.GetTask()
			c.logger.Infof("executing task: agentId=%s taskId=%s type=%s serviceKey=%s", c.cfg.AgentID, task.GetTaskId(), task.GetType().String(), task.GetServiceKey())
			go c.handleTask(ctx, task, outbound)
			continue
		}
		if msg.GetProxyConfig() != nil {
			c.logger.Infof("applying proxy snapshot: agentId=%s services=%d", c.cfg.AgentID, len(msg.GetProxyConfig().GetServices()))
			if err := c.proxy.ApplySnapshot(ctx, msg.GetProxyConfig()); err != nil {
				c.logger.Errorf(err, "failed to apply proxy snapshot: agentId=%s services=%d", c.cfg.AgentID, len(msg.GetProxyConfig().GetServices()))
				continue
			}
			continue
		}
		if msg.GetHaproxyConfigRequest() != nil {
			go c.handleHAProxyConfigRequest(ctx, msg.GetHaproxyConfigRequest(), outbound)
		}
	}
}

func (c *Client) handleTask(ctx context.Context, task *grpcapi.TaskCommand, outbound chan<- *grpcapi.AgentMessage) {
	if !c.state.TryStart(task.GetTaskId()) {
		c.logger.Infof("ignoring duplicated running task: agentId=%s taskId=%s", c.cfg.AgentID, task.GetTaskId())
		return
	}
	defer c.state.Done(task.GetTaskId())
	c.logger.Infof("running task executor: agentId=%s taskId=%s type=%s serviceKey=%s", c.cfg.AgentID, task.GetTaskId(), task.GetType().String(), task.GetServiceKey())
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
		var diagnostic *taskexec.TaskFailureDiagnostic
		if execErr, ok := err.(*taskexec.TaskExecutionError); ok && execErr.Step != "" {
			step = execErr.Step
			diagnostic = execErr.Diagnostic
		}
		update := &grpcapi.TaskUpdate{
			TaskId:       task.GetTaskId(),
			Status:       grpcapi.TaskStatus_TASK_STATUS_FAILED,
			Step:         step,
			ErrorMessage: err.Error(),
		}
		if diagnostic != nil {
			update.ContainerId = diagnostic.ContainerID
			update.DockerHealth = diagnostic.DockerHealth
			update.FailureLogs = diagnostic.FailureLogs
			update.CleanupCompleted = diagnostic.CleanupCompleted
		}
		outbound <- &grpcapi.AgentMessage{
			Payload: &grpcapi.AgentMessage_TaskUpdate{
				TaskUpdate: update,
			},
		}
		return
	}
	c.logger.Infof("waiting for next task: agentId=%s taskId=%s type=%s", c.cfg.AgentID, task.GetTaskId(), task.GetType().String())
}

func (c *Client) handleHAProxyConfigRequest(ctx context.Context, req *grpcapi.HAProxyConfigRequest, outbound chan<- *grpcapi.AgentMessage) {
	if req == nil {
		return
	}
	response := &grpcapi.HAProxyConfigResponse{
		RequestId: req.GetRequestId(),
		AgentId:   c.cfg.AgentID,
	}
	configText, err := c.proxy.ShowConfig(ctx)
	if err != nil {
		response.ErrorMessage = err.Error()
	} else {
		response.Config = configText
	}
	outbound <- &grpcapi.AgentMessage{
		Payload: &grpcapi.AgentMessage_HaproxyConfigResponse{
			HaproxyConfigResponse: response,
		},
	}
}
