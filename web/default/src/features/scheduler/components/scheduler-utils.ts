import type { UpsertSchedulerExecutorInput, UpsertSchedulerJobInput } from "../types";

export const scheduleKindLabel: Record<number, string> = {
  1: "一次性",
  2: "Cron",
};

export const dispatchPolicyLabel: Record<number, string> = {
  1: "轮询",
  2: "固定 live 槽",
};

export const runStatusLabel: Record<number, string> = {
  1: "待执行",
  2: "已派发",
  3: "执行中",
  4: "成功",
  5: "等待重试",
  6: "失败",
};

export function newJobForm(): UpsertSchedulerJobInput {
  return {
    name: "",
    taskType: "",
    payload: {},
    scheduleKind: "one_time",
    cronExpr: "*/5 * * * *",
    runAt: new Date().toISOString(),
    dispatchPolicy: "round_robin",
    executorGroup: "default",
    leaseTimeoutSec: 60,
    maxRetries: 3,
    metadata: {},
  };
}

export function newExecutorForm(): UpsertSchedulerExecutorInput {
  return {
    executorId: "",
    group: "default",
    enabled: true,
    metadata: {},
  };
}

export function parseMapJSON(text: string): Record<string, unknown> {
  if (!text.trim()) {
    return {};
  }
  const parsed = JSON.parse(text);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("必须是 JSON 对象");
  }
  return parsed as Record<string, unknown>;
}
