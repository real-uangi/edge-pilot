package controlplane

import (
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"edge-pilot/internal/shared/secret"
	"testing"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

func TestTaskToProtoPreservesFields(t *testing.T) {
	taskID := uuid.New()
	releaseID := uuid.New()
	serviceID := uuid.New()
	codec := secret.NewCodec(&config.ServiceSecretConfig{
		MasterKey:  []byte("12345678901234567890123456789012"),
		KeyVersion: "v1",
	})
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
			RegistryHost:      "ghcr.io",
			RegistryUsername:  "octocat",
			CommitSHA:         "abc123",
			TraceID:           "trace-1",
			TargetSlot:        model.SlotGreen,
			CurrentLiveSlot:   model.SlotBlue,
			ContainerPort:     8080,
			DockerHealthCheck: true,
			HTTPHealthPath:    "/health",
			HTTPExpectedCode:  200,
			HTTPTimeoutSecond: 5,
			BackendName:       "be_api",
			ServerName:        "srv_green",
			PreviousServer:    "srv_blue",
			Command:           []string{"run"},
			Entrypoint:        []string{"/bin/app"},
			Volumes: []model.VolumeMount{
				{Source: "/tmp/a", Target: "/data", ReadOnly: true},
			},
			PublishedPorts: []model.PublishedPort{
				{HostPort: 18081, ContainerPort: 8080},
			},
		}),
	}
	ciphertext, keyVersion, err := codec.EncryptJSON(model.TaskSensitivePayload{
		Env:            map[string]string{"A": "1"},
		RegistrySecret: "token-value",
	})
	if err != nil {
		t.Fatalf("EncryptJSON() error = %v", err)
	}
	task.SensitiveCiphertext = ciphertext
	task.SensitiveKeyVersion = keyVersion

	pb, err := taskToProto(task, codec)
	if err != nil {
		t.Fatalf("taskToProto() error = %v", err)
	}
	if pb.GetTaskId() != taskID.String() || pb.GetReleaseId() != releaseID.String() || pb.GetServiceId() != serviceID.String() {
		t.Fatalf("unexpected ids: %#v", pb)
	}
	if pb.GetType() != grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN {
		t.Fatalf("unexpected task type: %v", pb.GetType())
	}
	if pb.GetTargetSlot() != grpcapi.Slot_SLOT_GREEN || pb.GetCurrentLiveSlot() != grpcapi.Slot_SLOT_BLUE {
		t.Fatalf("unexpected slots: target=%v current=%v", pb.GetTargetSlot(), pb.GetCurrentLiveSlot())
	}
	if pb.GetContainerPort() != 8080 || !pb.GetDockerHealthCheck() {
		t.Fatalf("unexpected ports/health: container=%d health=%v", pb.GetContainerPort(), pb.GetDockerHealthCheck())
	}
	if pb.GetRegistryHost() != "ghcr.io" || pb.GetRegistryUsername() != "octocat" || pb.GetRegistrySecret() != "token-value" {
		t.Fatalf("unexpected registry credentials: %#v", pb)
	}
	if len(pb.GetPublishedPorts()) != 1 || pb.GetPublishedPorts()[0].GetHostPort() != 18081 {
		t.Fatalf("unexpected published ports: %#v", pb.GetPublishedPorts())
	}
	if len(pb.GetVolumes()) != 1 || pb.GetVolumes()[0].GetTarget() != "/data" || !pb.GetVolumes()[0].GetReadOnly() {
		t.Fatalf("unexpected volumes: %#v", pb.GetVolumes())
	}
}
