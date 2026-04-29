import { request } from "../../shared/lib/api-client";
import type {
  SchedulerJobRecord,
  SchedulerRunRecord,
  SchedulerExecutorRecord,
  UpsertSchedulerJobInput,
  UpsertSchedulerExecutorInput,
} from "./types";

export const schedulerApi = {
  listJobs() {
    return request<SchedulerJobRecord[]>("/api/admin/scheduler/jobs");
  },
  createJob(input: UpsertSchedulerJobInput) {
    return request<SchedulerJobRecord>("/api/admin/scheduler/jobs", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  updateJob(id: string, input: UpsertSchedulerJobInput) {
    return request<SchedulerJobRecord>(`/api/admin/scheduler/jobs/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  },
  deleteJob(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/scheduler/jobs/${id}`, {
      method: "DELETE",
    });
  },
  enableJob(id: string) {
    return request<SchedulerJobRecord>(`/api/admin/scheduler/jobs/${id}/enable`, { method: "POST", body: "{}" });
  },
  disableJob(id: string) {
    return request<SchedulerJobRecord>(`/api/admin/scheduler/jobs/${id}/disable`, { method: "POST", body: "{}" });
  },
  triggerJob(id: string, overridePayload?: Record<string, unknown>) {
    return request<SchedulerRunRecord>(`/api/admin/scheduler/jobs/${id}/trigger`, {
      method: "POST",
      body: JSON.stringify({ overridePayload: overridePayload ?? {} }),
    });
  },
  listRuns(id: string) {
    return request<SchedulerRunRecord[]>(`/api/admin/scheduler/jobs/${id}/runs`);
  },
  listExecutors() {
    return request<SchedulerExecutorRecord[]>("/api/admin/scheduler/executors");
  },
  createExecutor(input: UpsertSchedulerExecutorInput) {
    return request<SchedulerExecutorRecord>("/api/admin/scheduler/executors", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  resetExecutorToken(id: string) {
    return request<SchedulerExecutorRecord>(`/api/admin/scheduler/executors/${id}/reset-token`, {
      method: "POST",
      body: "{}",
    });
  },
  enableExecutor(id: string) {
    return request<SchedulerExecutorRecord>(`/api/admin/scheduler/executors/${id}/enable`, { method: "POST", body: "{}" });
  },
  disableExecutor(id: string) {
    return request<SchedulerExecutorRecord>(`/api/admin/scheduler/executors/${id}/disable`, { method: "POST", body: "{}" });
  },
  deleteExecutor(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/scheduler/executors/${id}`, { method: "DELETE" });
  },
};
