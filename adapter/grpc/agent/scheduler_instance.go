package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/real-uangi/edge-pilot/internal/agent/application/containerindex"

	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

const serviceInstanceMetadataKey = "edge_pilot_service_instance"

type schedulerInstanceConnector struct {
	cfg    *config.AgentRuntimeConfig
	docker agentdomain.DockerRuntime
	index  *containerindex.ManagedContainerIndex
	bridge *schedulerRelayBridge
	logger *log.StdLogger

	mu       sync.Mutex
	services map[string]*grpcapi.ProxyServiceConfig
	sessions map[string]schedulerInstanceSession
}

type schedulerInstanceSession struct {
	target schedulerInstanceTarget
	cancel context.CancelFunc
}

type schedulerInstanceTarget struct {
	executorID  string
	serviceID   string
	serviceKey  string
	releaseID   string
	containerID string
	slot        grpcapi.Slot
	group       string
	port        int
}

func (t schedulerInstanceTarget) equal(other schedulerInstanceTarget) bool {
	return t.executorID == other.executorID &&
		t.serviceID == other.serviceID &&
		t.serviceKey == other.serviceKey &&
		t.releaseID == other.releaseID &&
		t.containerID == other.containerID &&
		t.slot == other.slot &&
		t.group == other.group &&
		t.port == other.port
}

func newSchedulerInstanceConnector(cfg *config.AgentRuntimeConfig, docker agentdomain.DockerRuntime, index *containerindex.ManagedContainerIndex, bridge *schedulerRelayBridge) *schedulerInstanceConnector {
	return &schedulerInstanceConnector{
		cfg:      cfg,
		docker:   docker,
		index:    index,
		bridge:   bridge,
		logger:   log.NewStdLogger("agent.scheduler-instance"),
		services: make(map[string]*grpcapi.ProxyServiceConfig),
		sessions: make(map[string]schedulerInstanceSession),
	}
}

func (c *schedulerInstanceConnector) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.stopAll()
			return
		case <-ticker.C:
			c.reconcile(ctx)
		}
	}
}

func (c *schedulerInstanceConnector) UpdateSnapshot(snapshot *grpcapi.ProxyConfigSnapshot) {
	if snapshot == nil {
		return
	}
	next := make(map[string]*grpcapi.ProxyServiceConfig)
	for _, service := range snapshot.GetServices() {
		if service.GetSchedulerSdkPort() <= 0 || strings.TrimSpace(service.GetSchedulerExecutorGroup()) == "" {
			continue
		}
		next[service.GetServiceId()] = service
	}
	c.mu.Lock()
	c.services = next
	c.mu.Unlock()
}

func (c *schedulerInstanceConnector) reconcile(ctx context.Context) {
	if c.docker == nil || c.bridge == nil {
		return
	}
	services := c.snapshotServices()
	if len(services) == 0 {
		c.stopAll()
		return
	}
	containers, err := c.listContainers(ctx)
	if err != nil {
		c.logger.Errorf(err, "list managed containers for scheduler sdk failed")
		return
	}
	wanted := make(map[string]schedulerInstanceTarget)
	for _, container := range containers {
		if container == nil || strings.ToLower(strings.TrimSpace(container.State)) != "running" {
			continue
		}
		service := services[container.ServiceID]
		if service == nil || !isSchedulerReleaseContainer(container.ReleaseID, service) {
			continue
		}
		executorID := c.resolveExecutorID(ctx, container)
		if executorID == "" {
			continue
		}
		target := schedulerInstanceTarget{
			executorID:  executorID,
			serviceID:   container.ServiceID,
			serviceKey:  container.ServiceKey,
			releaseID:   container.ReleaseID,
			containerID: container.ContainerID,
			slot:        container.Slot,
			group:       service.GetSchedulerExecutorGroup(),
			port:        int(service.GetSchedulerSdkPort()),
		}
		wanted[target.executorID] = target
	}
	c.applyWanted(ctx, wanted)
}

func (c *schedulerInstanceConnector) resolveExecutorID(ctx context.Context, container *agentdomain.ManagedContainer) string {
	if container == nil {
		return ""
	}
	c.mu.Lock()
	for _, session := range c.sessions {
		if session.target.containerID == container.ContainerID {
			c.mu.Unlock()
			return session.target.executorID
		}
	}
	c.mu.Unlock()
	details, err := c.docker.GetContainerDetails(ctx, container.ContainerID)
	if err != nil {
		c.logger.Errorf(err, "get container details for executor id failed: containerId=%s", container.ContainerID)
		return ""
	}
	if details == nil || details.Env == nil {
		return ""
	}
	executorID := strings.TrimSpace(details.Env["EP_EXECUTOR_ID"])
	if executorID == "" {
		c.logger.Warnf("container missing EP_EXECUTOR_ID env: containerId=%s", container.ContainerID)
	}
	return executorID
}

func (c *schedulerInstanceConnector) listContainers(ctx context.Context) ([]*agentdomain.ManagedContainer, error) {
	if c.index == nil {
		return c.docker.ListManagedContainers(ctx, c.cfg.AgentID, "")
	}
	if err := c.index.RefreshNow(ctx); err != nil {
		return nil, err
	}
	return c.index.List(), nil
}

func (c *schedulerInstanceConnector) snapshotServices() map[string]*grpcapi.ProxyServiceConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]*grpcapi.ProxyServiceConfig, len(c.services))
	for key, value := range c.services {
		out[key] = value
	}
	return out
}

func (c *schedulerInstanceConnector) applyWanted(ctx context.Context, wanted map[string]schedulerInstanceTarget) {
	c.mu.Lock()
	for executorID, session := range c.sessions {
		target, ok := wanted[executorID]
		if !ok {
			session.cancel()
			delete(c.sessions, executorID)
			continue
		}
		if session.target.equal(target) {
			continue
		}
		session.cancel()
		delete(c.sessions, executorID)
	}
	for executorID, target := range wanted {
		if _, ok := c.sessions[executorID]; ok {
			continue
		}
		sessionCtx, cancel := context.WithCancel(ctx)
		c.sessions[executorID] = schedulerInstanceSession{target: target, cancel: cancel}
		if c.docker == nil || c.bridge == nil {
			continue
		}
		go c.connectInstance(sessionCtx, target)
	}
	c.mu.Unlock()
}

func (c *schedulerInstanceConnector) stopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for executorID, session := range c.sessions {
		session.cancel()
		delete(c.sessions, executorID)
	}
}

func (c *schedulerInstanceConnector) connectInstance(ctx context.Context, target schedulerInstanceTarget) {
	address, err := c.docker.ResolveListenAddress(ctx, target.containerID, target.port)
	if err != nil {
		c.logger.Errorf(err, "resolve scheduler sdk address failed: executorId=%s", target.executorID)
		c.forgetSession(target.executorID, target)
		return
	}
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		c.logger.Errorf(err, "connect scheduler sdk failed: executorId=%s addr=%s", target.executorID, address)
		c.forgetSession(target.executorID, target)
		return
	}
	defer conn.Close()

	client := grpcapi.NewSchedulerInstanceControlClient(conn)
	stream, err := client.Attach(ctx)
	if err != nil {
		c.logger.Errorf(err, "open scheduler sdk stream failed: executorId=%s addr=%s", target.executorID, address)
		c.forgetSession(target.executorID, target)
		return
	}

	session := &relayExecutorSession{
		id:         uuid.NewString(),
		executorID: target.executorID,
		routingKey: target.executorID,
		sendCh:     make(chan *grpcapi.SchedulerMessage, 16),
		done:       make(chan struct{}),
	}
	c.bridge.registerSession(session)
	defer func() {
		session.close()
		c.bridge.unregisterSession(session.id)
		c.sendSessionClosed(session, target)
		c.forgetSession(target.executorID, target)
	}()

	if err := c.sendServiceInstanceHello(session, target); err != nil {
		c.logger.Errorf(err, "send scheduler service instance hello failed: executorId=%s", target.executorID)
		return
	}

	var sendMu sync.Mutex
	if err := sendSchedulerInstanceMessage(stream, &sendMu, &grpcapi.SchedulerInstanceMessage{
		Payload: &grpcapi.SchedulerInstanceMessage_Attach{
			Attach: &grpcapi.SchedulerInstanceAttach{
				ExecutorId:  target.executorID,
				ServiceId:   target.serviceID,
				ServiceKey:  target.serviceKey,
				ReleaseId:   target.releaseID,
				Slot:        target.slot,
				Group:       target.group,
				ContainerId: target.containerID,
			},
		},
	}); err != nil {
		c.logger.Errorf(err, "send scheduler sdk attach failed: executorId=%s", target.executorID)
		return
	}

	go func() {
		for {
			select {
			case <-session.done:
				return
			case msg := <-session.sendCh:
				if err := sendSchedulerInstanceMessage(stream, &sendMu, schedulerMessageToInstanceMessage(msg)); err != nil {
					c.logger.Errorf(err, "send scheduler sdk downstream failed: executorId=%s", target.executorID)
					session.close()
					return
				}
			}
		}
	}()

	for {
		msg, recvErr := stream.Recv()
		if recvErr != nil {
			c.logger.Errorf(recvErr, "scheduler sdk stream closed: executorId=%s", target.executorID)
			return
		}
		executorMsg := schedulerInstanceMessageToExecutorMessage(target.executorID, msg)
		if executorMsg == nil {
			continue
		}
		if err := c.sendExecutorMessage(session, target, executorMsg); err != nil {
			c.logger.Errorf(err, "send scheduler sdk upstream failed: executorId=%s", target.executorID)
			return
		}
	}
}

func (c *schedulerInstanceConnector) forgetSession(executorID string, target schedulerInstanceTarget) {
	c.mu.Lock()
	defer c.mu.Unlock()
	session, ok := c.sessions[executorID]
	if !ok {
		return
	}
	if !session.target.equal(target) {
		return
	}
	delete(c.sessions, executorID)
}

func (c *schedulerInstanceConnector) sendServiceInstanceHello(session *relayExecutorSession, target schedulerInstanceTarget) error {
	return c.sendExecutorMessage(session, target, &grpcapi.ExecutorMessage{
		Payload: &grpcapi.ExecutorMessage_Hello{
			Hello: &grpcapi.ExecutorHello{
				ExecutorId: target.executorID,
				Group:      target.group,
				LiveSlot:   target.slot,
				Metadata: map[string]string{
					serviceInstanceMetadataKey: "true",
					"service_id":               target.serviceID,
					"service_key":              target.serviceKey,
					"release_id":               target.releaseID,
					"container_id":             target.containerID,
					"executor_id":              target.executorID,
				},
				RoutingKey: target.executorID,
			},
		},
	})
}

func (c *schedulerInstanceConnector) sendExecutorMessage(session *relayExecutorSession, target schedulerInstanceTarget, msg *grpcapi.ExecutorMessage) error {
	payloadBytes, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.bridge.sendUpstream(&grpcapi.AgentMessage{Payload: &grpcapi.AgentMessage_SchedulerEnvelope{SchedulerEnvelope: &grpcapi.SchedulerRelayEnvelope{
		RelaySessionId: session.id,
		ExecutorId:     target.executorID,
		RoutingKey:     target.executorID,
		AgentId:        c.cfg.AgentID,
		Direction:      grpcapi.SchedulerRelayDirection_SCHEDULER_RELAY_DIRECTION_UPSTREAM,
		PayloadType:    grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_EXECUTOR_MESSAGE,
		PayloadBytes:   payloadBytes,
	}}})
}

func (c *schedulerInstanceConnector) sendSessionClosed(session *relayExecutorSession, target schedulerInstanceTarget) {
	_ = c.bridge.sendUpstream(&grpcapi.AgentMessage{Payload: &grpcapi.AgentMessage_SchedulerEnvelope{SchedulerEnvelope: &grpcapi.SchedulerRelayEnvelope{
		RelaySessionId: session.id,
		ExecutorId:     target.executorID,
		RoutingKey:     target.executorID,
		AgentId:        c.cfg.AgentID,
		Direction:      grpcapi.SchedulerRelayDirection_SCHEDULER_RELAY_DIRECTION_CONTROL,
		PayloadType:    grpcapi.SchedulerRelayPayloadType_SCHEDULER_RELAY_PAYLOAD_TYPE_SESSION_CLOSED,
	}}})
}

func sendSchedulerInstanceMessage(stream grpcapi.SchedulerInstanceControl_AttachClient, mu *sync.Mutex, msg *grpcapi.SchedulerInstanceMessage) error {
	if msg == nil {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	return stream.Send(msg)
}

func schedulerMessageToInstanceMessage(msg *grpcapi.SchedulerMessage) *grpcapi.SchedulerInstanceMessage {
	if msg == nil {
		return nil
	}
	if msg.GetAck() != nil {
		return &grpcapi.SchedulerInstanceMessage{Payload: &grpcapi.SchedulerInstanceMessage_Ack{Ack: msg.GetAck()}}
	}
	if msg.GetRun() != nil {
		return &grpcapi.SchedulerInstanceMessage{Payload: &grpcapi.SchedulerInstanceMessage_Run{Run: msg.GetRun()}}
	}
	return nil
}

func schedulerInstanceMessageToExecutorMessage(executorID string, msg *grpcapi.SchedulerInstanceMessage) *grpcapi.ExecutorMessage {
	if msg == nil {
		return nil
	}
	switch {
	case msg.GetHeartbeat() != nil:
		heartbeat := msg.GetHeartbeat()
		if heartbeat.ExecutorId == "" {
			heartbeat.ExecutorId = executorID
		}
		return &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_Heartbeat{Heartbeat: heartbeat}}
	case msg.GetRunUpdate() != nil:
		return &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_RunUpdate{RunUpdate: msg.GetRunUpdate()}}
	case msg.GetLeaseRenew() != nil:
		return &grpcapi.ExecutorMessage{Payload: &grpcapi.ExecutorMessage_LeaseRenew{LeaseRenew: msg.GetLeaseRenew()}}
	default:
		return nil
	}
}

func isSchedulerReleaseContainer(releaseID string, service *grpcapi.ProxyServiceConfig) bool {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return false
	}
	return releaseID == strings.TrimSpace(service.GetLiveReleaseId()) || releaseID == strings.TrimSpace(service.GetCandidateReleaseId())
}

func schedulerInstanceExecutorID(serviceID string, releaseID string, slot grpcapi.Slot, containerID string) string {
	serviceID = strings.TrimSpace(serviceID)
	releaseID = strings.TrimSpace(releaseID)
	containerID = strings.TrimSpace(containerID)
	if serviceID == "" || releaseID == "" || containerID == "" {
		return ""
	}
	return fmt.Sprintf("svc:%s:rel:%s:slot:%s:ctr:%s", serviceID, releaseID, schedulerSlotToken(slot), shortContainerID(containerID))
}

func schedulerSlotToken(slot grpcapi.Slot) string {
	switch slot {
	case grpcapi.Slot_SLOT_BLUE:
		return "blue"
	case grpcapi.Slot_SLOT_GREEN:
		return "green"
	default:
		return "unknown"
	}
}

func shortContainerID(containerID string) string {
	containerID = strings.TrimSpace(containerID)
	if len(containerID) <= 12 {
		return containerID
	}
	return containerID[:12]
}
