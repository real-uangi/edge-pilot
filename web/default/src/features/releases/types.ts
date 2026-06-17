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
  releaseNotesAggregate: ReleaseNotesItem[];
}
