package controlplane

import (
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"testing"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

func TestTaskToProtoPreservesFields(t *testing.T) {
	taskID := uuid.New()
	releaseID := uuid.New()
	serviceID := uuid.New()
	task := &model.Task{
		ID:        taskID,
		ReleaseID: releaseID,
		ServiceID: serviceID,
		AgentID:   "agent-a",
		Type:      model.TaskTypeDeployGreen,
		Payload: commondb.NewJSONB(model.TaskPayload{
			ServiceID:         serviceID,
			ServiceKey:        "svc-a",
			ImageRepo:         "repo/app",
			ImageTag:          "v1.2.3",
			CommitSHA:         "abc123",
			TraceID:           "trace-1",
			TargetSlot:        model.SlotGreen,
			CurrentLiveSlot:   model.SlotBlue,
			ContainerPort:     8080,
			HostPort:          18081,
			HTTPHealthPath:    "/health",
			HTTPExpectedCode:  200,
			HTTPTimeoutSecond: 5,
			BackendName:       "be_api",
			ServerName:        "srv_green",
			PreviousServer:    "srv_blue",
			Env:               map[string]string{"A": "1"},
			Command:           []string{"run"},
			Entrypoint:        []string{"/bin/app"},
			Volumes: []model.VolumeMount{
				{Source: "/tmp/a", Target: "/data", ReadOnly: true},
			},
		}),
	}

	pb := taskToProto(task)
	if pb.GetTaskId() != taskID.String() || pb.GetReleaseId() != releaseID.String() || pb.GetServiceId() != serviceID.String() {
		t.Fatalf("unexpected ids: %#v", pb)
	}
	if pb.GetType() != grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN {
		t.Fatalf("unexpected task type: %v", pb.GetType())
	}
	if pb.GetTargetSlot() != grpcapi.Slot_SLOT_GREEN || pb.GetCurrentLiveSlot() != grpcapi.Slot_SLOT_BLUE {
		t.Fatalf("unexpected slots: target=%v current=%v", pb.GetTargetSlot(), pb.GetCurrentLiveSlot())
	}
	if pb.GetHostPort() != 18081 || pb.GetContainerPort() != 8080 {
		t.Fatalf("unexpected ports: host=%d container=%d", pb.GetHostPort(), pb.GetContainerPort())
	}
	if len(pb.GetVolumes()) != 1 || pb.GetVolumes()[0].GetTarget() != "/data" || !pb.GetVolumes()[0].GetReadOnly() {
		t.Fatalf("unexpected volumes: %#v", pb.GetVolumes())
	}
}
