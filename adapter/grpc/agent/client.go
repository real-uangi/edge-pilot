package agent

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/real-uangi/edge-pilot/internal/agent/application/containerindex"
	"github.com/real-uangi/edge-pilot/internal/agent/application/taskexec"
	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
	"github.com/real-uangi/edge-pilot/internal/shared/perf"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	cfg        *config.AgentRuntimeConfig
	executor   *taskexec.Executor
	docker     agentdomain.DockerRuntime
	proxy      agentdomain.ProxyRuntime
	state      *taskexec.RuntimeState
	collector  perf.Collector
	relay      *schedulerRelayBridge
	instances  *schedulerInstanceConnector
	logger     *log.StdLogger
	logStreams map[string]*logStreamState
	logMu      sync.Mutex
}

func NewClient(cfg *config.AgentRuntimeConfig, executor *taskexec.Executor, docker agentdomain.DockerRuntime, proxy agentdomain.ProxyRuntime, state *taskexec.RuntimeState, collector perf.Collector, index *containerindex.ManagedContainerIndex) *Client {
	relay := newSchedulerRelayBridge(cfg)
	return &Client{
		cfg:       cfg,
		executor:  executor,
		docker:    docker,
		proxy:     proxy,
		state:     state,
		collector: collector,
		relay:     relay,
		instances: newSchedulerInstanceConnector(cfg, docker, index, relay),
		logger:    log.NewStdLogger("agent.grpc-client"),
	}
}

func startClient(lc fx.Lifecycle, client *Client) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if err := client.relay.Start(); err != nil {
				return err
			}
			go client.instances.Start(ctx)
			client.logger.Infof("starting grpc client after proxy stack preparation: agentId=%s addr=%s", client.cfg.AgentID, client.cfg.ControlPlaneAddr)
			go client.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			client.relay.Stop(context.Background())
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
	c.relay.SetOutbound(outbound)
	defer c.relay.clearOutbound()
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
			c.instances.UpdateSnapshot(msg.GetProxyConfig())
			continue
		}
		if msg.GetHaproxyConfigRequest() != nil {
			go c.handleHAProxyConfigRequest(ctx, msg.GetHaproxyConfigRequest(), outbound)
		}
		if msg.GetContainerListRequest() != nil {
			go c.handleContainerListRequest(ctx, msg.GetContainerListRequest(), outbound)
		}
		if msg.GetContainerInspectRequest() != nil {
			go c.handleContainerInspectRequest(ctx, msg.GetContainerInspectRequest(), outbound)
		}
		if msg.GetContainerLogStreamRequest() != nil {
			req := msg.GetContainerLogStreamRequest()
			if req.GetFollow() {
				go c.startLogStream(ctx, req, outbound)
			} else {
				c.stopLogStream(req.GetContainerId())
			}
		}
		if msg.GetSchedulerEnvelope() != nil {
			c.relay.HandleControlEnvelope(msg.GetSchedulerEnvelope())
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

func (c *Client) handleContainerListRequest(ctx context.Context, req *grpcapi.ContainerListRequest, outbound chan<- *grpcapi.AgentMessage) {
	if req == nil {
		return
	}
	containers, err := c.docker.ListManagedContainers(ctx, c.cfg.AgentID, "")
	response := &grpcapi.ContainerListResponse{
		RequestId: req.GetRequestId(),
		AgentId:   c.cfg.AgentID,
	}
	if err != nil {
		response.ErrorMessage = err.Error()
	} else {
		summaries := make([]*grpcapi.ContainerSummary, 0, len(containers))
		for _, container := range containers {
			if container == nil {
				continue
			}
			summaries = append(summaries, &grpcapi.ContainerSummary{
				ContainerId: container.ContainerID,
				Name:        container.Name,
				State:       container.State,
				Image:       container.Image,
				ServiceId:   container.ServiceID,
				ServiceKey:  container.ServiceKey,
				ReleaseId:   container.ReleaseID,
				Slot:        container.Slot,
				CreatedAt:   container.CreatedAt,
			})
		}
		response.Containers = summaries
	}
	outbound <- &grpcapi.AgentMessage{
		Payload: &grpcapi.AgentMessage_ContainerListResponse{
			ContainerListResponse: response,
		},
	}
}

func (c *Client) handleContainerInspectRequest(ctx context.Context, req *grpcapi.ContainerInspectRequest, outbound chan<- *grpcapi.AgentMessage) {
	if req == nil {
		return
	}
	response := &grpcapi.ContainerInspectResponse{
		RequestId: req.GetRequestId(),
		AgentId:   c.cfg.AgentID,
	}

	details, err := c.docker.GetContainerDetails(ctx, req.GetContainerId())
	if err != nil {
		c.logger.Errorf(err, "container inspect failed: containerId=%s", req.GetContainerId())
		if strings.Contains(err.Error(), "404") {
			response.ErrorMessage = "container not found"
		} else {
			response.ErrorMessage = "container inspect failed"
		}
		outbound <- &grpcapi.AgentMessage{
			Payload: &grpcapi.AgentMessage_ContainerInspectResponse{
				ContainerInspectResponse: response,
			},
		}
		return
	}

	response.Details = &grpcapi.ContainerDetails{
		ContainerId:  details.ContainerID,
		Name:         details.Name,
		State:        details.State,
		Image:        details.Image,
		Running:      details.Running,
		Health:       details.Health,
		RestartCount: details.RestartCount,
		Labels:       details.Labels,
		Env:          details.Env,
		Command:      details.Command,
		Entrypoint:   details.Entrypoint,
		IpAddress:    details.IPAddress,
		CpuLimit:     details.CPULimit,
		MemoryLimit:  details.MemoryLimit,
		CreatedAt:    details.CreatedAt,
	}
	for _, v := range details.Volumes {
		response.Details.Volumes = append(response.Details.Volumes, &grpcapi.VolumeMount{
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}
	for _, p := range details.Ports {
		response.Details.Ports = append(response.Details.Ports, &grpcapi.PublishedPort{
			ContainerPort: p.ContainerPort,
			HostPort:      p.HostPort,
		})
	}
	outbound <- &grpcapi.AgentMessage{
		Payload: &grpcapi.AgentMessage_ContainerInspectResponse{
			ContainerInspectResponse: response,
		},
	}
}

type logStreamState struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (c *Client) startLogStream(ctx context.Context, req *grpcapi.ContainerLogStreamRequest, outbound chan<- *grpcapi.AgentMessage) {
	streamCtx, cancel := context.WithCancel(ctx)
	c.logMu.Lock()
	if c.logStreams == nil {
		c.logStreams = make(map[string]*logStreamState)
	}
	if existing, ok := c.logStreams[req.GetContainerId()]; ok {
		existing.cancel()
		<-existing.done
	}
	done := make(chan struct{})
	c.logStreams[req.GetContainerId()] = &logStreamState{cancel: cancel, done: done}
	c.logMu.Unlock()

	go func() {
		defer func() {
			recover()
		}()
		defer close(done)
		defer cancel()
		defer func() {
			c.logMu.Lock()
			if state, ok := c.logStreams[req.GetContainerId()]; ok && state.done == done {
				delete(c.logStreams, req.GetContainerId())
			}
			c.logMu.Unlock()
		}()
		reader, err := c.docker.StreamContainerLogs(streamCtx, req.GetContainerId(), int(req.GetTailLines()), true, true, true)
		if err != nil {
			c.logger.Errorf(err, "failed to start log stream: containerId=%s", req.GetContainerId())
			return
		}
		defer reader.Close()

		for {
			select {
			case <-streamCtx.Done():
				return
			default:
			}

			data, stderr, err := readDockerLogFrame(reader)
			if err != nil {
				if err != io.EOF {
					c.logger.Errorf(err, "log stream read error: containerId=%s", req.GetContainerId())
				}
				return
			}

			select {
			case <-streamCtx.Done():
				return
			case outbound <- &grpcapi.AgentMessage{
				Payload: &grpcapi.AgentMessage_ContainerLogChunk{
					ContainerLogChunk: &grpcapi.ContainerLogChunk{
						RequestId:   req.GetRequestId(),
						AgentId:     c.cfg.AgentID,
						ContainerId: req.GetContainerId(),
						Data:        data,
						Stderr:      stderr,
					},
				},
			}:
				// sent successfully
			}
		}
	}()
}

func (c *Client) stopLogStream(containerID string) {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	if c.logStreams == nil {
		return
	}
	if state, ok := c.logStreams[containerID]; ok {
		state.cancel()
		delete(c.logStreams, containerID)
	}
}

const maxLogFrameSize = 1024 * 1024 // 1MB

func readDockerLogFrame(reader io.Reader) ([]byte, bool, error) {
	header := make([]byte, 8)
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, false, err
	}

	streamType := header[0]
	size := binary.BigEndian.Uint32(header[4:8])
	if size > maxLogFrameSize {
		return nil, false, fmt.Errorf("log frame too large: %d bytes", size)
	}

	data := make([]byte, size)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, false, err
	}

	return data, streamType == 2, nil
}
