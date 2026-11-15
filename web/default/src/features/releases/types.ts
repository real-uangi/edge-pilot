export interface CreateReleaseInput {
  serviceId: string;
  imageTag: string;
  releaseNotes?: string;
}

export interface ReleaseRecord {
  id: string;
  serviceId: string;
  agentId: string;
  imageRepo: string;
  imageTag: string;
  commitSha: string;
  triggeredBy: string;
  traceId: string;
  status: number;
  trafficPercent: number;
  targetSlot: number;
  previousLiveSlot: number;
  currentTaskId: string | null;
  switchConfirmed: boolean | null;
  verificationUrl: string;
  stickyCookieName: string;
  stickyCookieTtl: number;
  currentReleaseHeaderName: string;
  liveReleaseHeaderName: string;
  releaseNotes: string;
  isActive: boolean;
  queuePosition: number;
  createdAt: string;
  updatedAt: string;
  completedAt: string | null;
}

export interface TaskSnapshot {
  id: string;
  type: number;
  status: number;
  lastError: string;
  lastStep: string;
  dockerHealth: string;
  failureLogs: string;
  cleanupCompleted: boolean | null;
  createdAt: string;
  dispatchedAt: string | null;
  startedAt: string | null;
  completedAt: string | null;
  attempts: TaskAttempt[];
}

export interface TaskAttempt {
  id: string;
  status: number;
  message: string;
  startedAt: string | null;
  completedAt: string | null;
}

export interface AuditLog {
  id: string;
  eventType: string;
  message: string;
  createdAt: string;
}

export interface ReleaseNotesItem {
  id: string;
  imageTag: string;
  releaseNotes: string;
  createdAt: string;
}

export interface ReleaseDetail {
  release: ReleaseRecord;
  tasks: TaskSnapshot[];
  audits: AuditLog[];
  pipeline: ReleasePipeline;
  releaseNotesAggregate: ReleaseNotesItem[];
}

export interface ReleasePipeline {
  stages: PipelineStage[];
  runs: PipelineRun[];
}

export interface PipelineStage {
  id: string;
  label: string;
  kind: "automatic" | "manual" | "event";
  estimatedDurationMs: number | null;
  sampleCount: number;
}

export interface PipelineRun {
  taskId: string;
  sequence: number;
  status: number;
  startedAt: string;
  completedAt: string | null;
  replayCount: number;
  cells: PipelineCell[];
}

export interface PipelineCell {
  stageId: string;
  status:
    | "pending"
    | "running"
    | "completed"
    | "failed"
    | "timed_out"
    | "waiting"
    | "unrecorded"
    | "skipped";
  durationMs: number | null;
  runningSince: string | null;
  events: PipelineEvent[];
}

export interface PipelineEvent {
  id: string;
  kind: "task" | "audit";
  code: string;
  message: string;
  at: string;
}
