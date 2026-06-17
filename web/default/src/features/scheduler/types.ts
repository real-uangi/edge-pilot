export interface SchedulerJobRecord {
  id: string;
  name: string;
  handlerKey: string;
  serviceId: string | null;
  payload: Record<string, unknown>;
  scheduleKind: number;
  cronExpr: string;
  runAt: string | null;
  nextRunAt: string | null;
  enabled: boolean | null;
  dispatchPolicy: number;
  executorGroup: string;
  leaseTimeoutSec: number;
  maxRetries: number;
  metadata: Record<string, string>;
  createdAt: string;
  updatedAt: string;
}

export interface SchedulerRunRecord {
  id: string;
  jobId: string;
  handlerKey: string;
  serviceId: string | null;
  payload: Record<string, unknown>;
  status: number;
  attempt: number;
  maxRetries: number;
  scheduledAt: string;
  dispatchedAt: string | null;
  startedAt: string | null;
  completedAt: string | null;
  leaseExpiresAt: string | null;
  leasedBy: string;
  nextRetryAt: string | null;
  errorMessage: string;
  createdAt: string;
  updatedAt: string;
}

export interface SchedulerExecutorRecord {
  id: string;
  group: string;
  channelMode: number;
  relayAgentId: string;
  relayRoutingKey: string;
  enabled: boolean | null;
  lastSeenAt: string | null;
  liveSlot: number;
  instanceMeta: Record<string, string>;
  token?: string;
  createdAt: string;
  updatedAt: string;
}

export interface UpsertSchedulerJobInput {
  name: string;
  handlerKey: string;
  serviceId: string;
  payload: Record<string, unknown>;
  scheduleKind: "one_time" | "cron";
  cronExpr?: string;
  runAt?: string | null;
  enabled?: boolean;
  dispatchPolicy?: "round_robin" | "fixed_live_slot";
  executorGroup: string;
  leaseTimeoutSec?: number;
  maxRetries?: number;
  metadata?: Record<string, string>;
}

export interface UpsertSchedulerExecutorInput {
  executorId: string;
  group: string;
  enabled?: boolean;
  liveSlot?: number;
  metadata?: Record<string, string>;
}
