package controlplane

import (
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"edge-pilot/internal/shared/secret"
	"strings"

	"github.com/real-uangi/allingo/common/business"
)

func taskToProto(task *model.Task, codec *secret.Codec) (*grpcapi.TaskCommand, error) {
	payload := getPayload(task)
	sensitive, err := getSensitivePayload(task, codec)
	if err != nil {
		return nil, err
	}
	return &grpcapi.TaskCommand{
		TaskId:            task.ID.String(),
		ReleaseId:         task.ReleaseID.String(),
		ServiceId:         payload.ServiceID.String(),
		ServiceKey:        payload.ServiceKey,
		AgentId:           task.AgentID,
		Type:              toProtoTaskType(task.Type),
		ImageRepo:         payload.ImageRepo,
		ImageTag:          payload.ImageTag,
		RegistryHost:      payload.RegistryHost,
		RegistryUsername:  payload.RegistryUsername,
		RegistrySecret:    firstNonEmpty(sensitive.RegistrySecret, payload.RegistrySecret),
		CommitSha:         payload.CommitSHA,
		TraceId:           payload.TraceID,
		TargetSlot:        toProtoSlot(payload.TargetSlot),
		CurrentLiveSlot:   toProtoSlot(payload.CurrentLiveSlot),
		ContainerPort:     int32(payload.ContainerPort),
		DockerHealthCheck: payload.DockerHealthCheck,
		HttpHealthPath:    payload.HTTPHealthPath,
		HttpExpectedCode:  int32(payload.HTTPExpectedCode),
		HttpTimeoutSecond: int32(payload.HTTPTimeoutSecond),
		BackendName:       payload.BackendName,
		ServerName:        payload.ServerName,
		PreviousServer:    payload.PreviousServer,
		Env:               firstNonEmptyMap(sensitive.Env, payload.Env),
		Command:           payload.Command,
		Entrypoint:        payload.Entrypoint,
		Volumes:           toProtoVolumes(payload.Volumes),
		PublishedPorts:    toProtoPublishedPorts(payload.PublishedPorts),
	}, nil
}

func getSensitivePayload(task *model.Task, codec *secret.Codec) (model.TaskSensitivePayload, error) {
	if strings.TrimSpace(task.SensitiveCiphertext) == "" {
		return model.TaskSensitivePayload{}, nil
	}
	if codec == nil {
		return model.TaskSensitivePayload{}, business.NewErrorWithCode("service secret master key not configured", 500)
	}
	var sensitive model.TaskSensitivePayload
	if err := codec.DecryptJSON(task.SensitiveCiphertext, task.SensitiveKeyVersion, &sensitive); err != nil {
		return model.TaskSensitivePayload{}, err
	}
	return sensitive, nil
}

func firstNonEmpty(current string, fallback string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return fallback
}

func firstNonEmptyMap(current map[string]string, fallback map[string]string) map[string]string {
	if len(current) > 0 {
		return current
	}
	return fallback
}

func toProtoTaskType(taskType model.TaskType) grpcapi.TaskType {
	switch taskType {
	case model.TaskTypeDeployGreen:
		return grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN
	case model.TaskTypeSwitchTraffic:
		return grpcapi.TaskType_TASK_TYPE_SWITCH_TRAFFIC
	case model.TaskTypeRollback:
		return grpcapi.TaskType_TASK_TYPE_ROLLBACK
	case model.TaskTypeCleanupOld:
		return grpcapi.TaskType_TASK_TYPE_CLEANUP_OLD
	default:
		return grpcapi.TaskType_TASK_TYPE_UNSPECIFIED
	}
}

func fromProtoTaskStatus(status grpcapi.TaskStatus) string {
	switch status {
	case grpcapi.TaskStatus_TASK_STATUS_RUNNING:
		return "running"
	case grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED:
		return "succeeded"
	case grpcapi.TaskStatus_TASK_STATUS_FAILED:
		return "failed"
	default:
		return ""
	}
}

func toProtoTaskStatus(status string) grpcapi.TaskStatus {
	switch status {
	case "running":
		return grpcapi.TaskStatus_TASK_STATUS_RUNNING
	case "succeeded":
		return grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED
	case "failed":
		return grpcapi.TaskStatus_TASK_STATUS_FAILED
	default:
		return grpcapi.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

func toProtoSlot(slot model.Slot) grpcapi.Slot {
	switch slot {
	case model.SlotBlue:
		return grpcapi.Slot_SLOT_BLUE
	case model.SlotGreen:
		return grpcapi.Slot_SLOT_GREEN
	default:
		return grpcapi.Slot_SLOT_UNSPECIFIED
	}
}

func fromProtoSlot(slot grpcapi.Slot) model.Slot {
	switch slot {
	case grpcapi.Slot_SLOT_BLUE:
		return model.SlotBlue
	case grpcapi.Slot_SLOT_GREEN:
		return model.SlotGreen
	default:
		return 0
	}
}

func toProtoVolumes(items []model.VolumeMount) []*grpcapi.VolumeMount {
	out := make([]*grpcapi.VolumeMount, 0, len(items))
	for _, item := range items {
		out = append(out, &grpcapi.VolumeMount{
			Source:   item.Source,
			Target:   item.Target,
			ReadOnly: item.ReadOnly,
		})
	}
	return out
}

func toProtoPublishedPorts(items []model.PublishedPort) []*grpcapi.PublishedPort {
	out := make([]*grpcapi.PublishedPort, 0, len(items))
	for _, item := range items {
		out = append(out, &grpcapi.PublishedPort{
			HostPort:      int32(item.HostPort),
			ContainerPort: int32(item.ContainerPort),
		})
	}
	return out
}
