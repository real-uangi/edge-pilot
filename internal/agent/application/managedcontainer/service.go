package managedcontainer

import (
	"context"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/grpcapi"
	"sync"
	"time"

	"github.com/real-uangi/allingo/common/log"
)

type AgentLister interface {
	List() ([]dto.AgentOverview, error)
}

type ContainerRuntimeRequester interface {
	RequestContainerList(ctx context.Context, agentID string) ([]*grpcapi.ContainerSummary, error)
	RequestContainerInspect(ctx context.Context, agentID, containerID string) (*grpcapi.ContainerDetails, error)
	StartContainerLogStream(ctx context.Context, agentID, containerID string, tailLines int32) (chan *grpcapi.ContainerLogChunk, error)
	StopContainerLogStream(agentID, containerID string) error
}

type ManagedContainerService struct {
	agents    AgentLister
	requester ContainerRuntimeRequester
	logger    *log.StdLogger
}

func NewManagedContainerService(agents AgentLister, requester ContainerRuntimeRequester) *ManagedContainerService {
	return &ManagedContainerService{
		agents:    agents,
		requester: requester,
		logger:    log.NewStdLogger("agent.managed-container"),
	}
}

func (s *ManagedContainerService) ListContainers() ([]dto.ManagedInstanceOutput, error) {
	agentList, err := s.agents.List()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	resultCh := make(chan []dto.ManagedInstanceOutput, len(agentList))

	for _, agent := range agentList {
		if agent.Online == nil || !*agent.Online {
			continue
		}
		wg.Add(1)
		go func(agent dto.AgentOverview) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			summaries, err := s.requester.RequestContainerList(ctx, agent.ID)
			if err != nil {
				s.logger.Warnf("failed to list containers for agent %s: %v", agent.ID, err)
				return
			}

			instances := make([]dto.ManagedInstanceOutput, 0, len(summaries))
			for _, summary := range summaries {
				instances = append(instances, dto.ManagedInstanceOutput{
					AgentID:     agent.ID,
					AgentHost:   agent.Hostname,
					ContainerID: summary.GetContainerId(),
					Name:        summary.GetName(),
					State:       summary.GetState(),
					Image:       summary.GetImage(),
					ServiceID:   summary.GetServiceId(),
					ServiceKey:  summary.GetServiceKey(),
					ReleaseID:   summary.GetReleaseId(),
					Slot:        slotToString(summary.GetSlot()),
					CreatedAt:   summary.GetCreatedAt(),
				})
			}
			resultCh <- instances
		}(agent)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []dto.ManagedInstanceOutput
	for instances := range resultCh {
		results = append(results, instances...)
	}

	return results, nil
}

func (s *ManagedContainerService) GetContainerDetails(agentID, containerID string) (*dto.ManagedInstanceDetailsOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	details, err := s.requester.RequestContainerInspect(ctx, agentID, containerID)
	if err != nil {
		return nil, err
	}

	return &dto.ManagedInstanceDetailsOutput{
		ManagedInstanceOutput: dto.ManagedInstanceOutput{
			AgentID:     agentID,
			ContainerID: details.GetContainerId(),
			Name:        details.GetName(),
			State:       details.GetState(),
			Image:       details.GetImage(),
			ServiceID:   details.GetServiceId(),
			ServiceKey:  details.GetServiceKey(),
			ReleaseID:   details.GetReleaseId(),
			Slot:        slotToString(details.GetSlot()),
			CreatedAt:   details.GetCreatedAt(),
		},
		Running:      details.GetRunning(),
		Health:       details.GetHealth(),
		RestartCount: int(details.GetRestartCount()),
		IPAddress:    details.GetIpAddress(),
		Labels:       details.GetLabels(),
		Env:          details.GetEnv(),
		Command:      details.GetCommand(),
		Entrypoint:   details.GetEntrypoint(),
		Volumes:      convertProtoVolumes(details.GetVolumes()),
		Ports:        convertProtoPorts(details.GetPorts()),
		CPULimit:     details.GetCpuLimit(),
		MemoryLimit:  details.GetMemoryLimit(),
	}, nil
}

func (s *ManagedContainerService) StreamContainerLogs(ctx context.Context, agentID, containerID string, writer func(data string, stderr bool) error) error {
	chunkCh, err := s.requester.StartContainerLogStream(ctx, agentID, containerID, 100)
	if err != nil {
		return err
	}
	defer s.requester.StopContainerLogStream(agentID, containerID)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-chunkCh:
			if !ok {
				return nil
			}
			if err := writer(string(chunk.GetData()), chunk.GetStderr()); err != nil {
				return err
			}
		}
	}
}

func slotToString(slot grpcapi.Slot) string {
	switch slot {
	case grpcapi.Slot_SLOT_BLUE:
		return "blue"
	case grpcapi.Slot_SLOT_GREEN:
		return "green"
	default:
		return "unknown"
	}
}

func convertProtoVolumes(volumes []*grpcapi.VolumeMount) []dto.VolumeMount {
	result := make([]dto.VolumeMount, 0, len(volumes))
	for _, v := range volumes {
		result = append(result, dto.VolumeMount{
			Source:   v.GetSource(),
			Target:   v.GetTarget(),
			ReadOnly: v.GetReadOnly(),
		})
	}
	return result
}

func convertProtoPorts(ports []*grpcapi.PublishedPort) []dto.PublishedPort {
	result := make([]dto.PublishedPort, 0, len(ports))
	for _, p := range ports {
		result = append(result, dto.PublishedPort{
			HostPort:      int(p.GetHostPort()),
			ContainerPort: int(p.GetContainerPort()),
		})
	}
	return result
}
