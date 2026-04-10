package controlplane

import (
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
)

func taskToProto(task *model.Task) *grpcapi.TaskCommand {
	payload := getPayload(task)
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
		RegistrySecret:    payload.RegistrySecret,
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
		Env:               payload.Env,
		Command:           payload.Command,
		Entrypoint:        payload.Entrypoint,
		Volumes:           toProtoVolumes(payload.Volumes),
		PublishedPorts:    toProtoPublishedPorts(payload.PublishedPorts),
	}
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
