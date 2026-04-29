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
}

export interface ReleaseDetail {
  release: ReleaseRecord;
  tasks: TaskSnapshot[];
}
