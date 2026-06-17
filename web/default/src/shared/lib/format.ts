export function formatDateTime(value?: string | null): string {
  if (!value) {
    return "—";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value));
}

export function shortId(value: string): string {
  return value.slice(0, 8);
}

type AgentDisplayInput = {
  id: string;
  hostname?: string | null;
  ip?: string | null;
};

export function formatAgentLabel(agent: AgentDisplayInput): string {
  const parts = [agent.hostname?.trim(), agent.ip?.trim(), shortId(agent.id)].filter(Boolean);
  return parts.join(" · ");
}

export function boolLabel(value?: boolean | null, trueText = "是", falseText = "否"): string {
  if (value == null) {
    return "未知";
  }
  return value ? trueText : falseText;
}

export function formatPercent(value?: number | null): string {
  if (value == null || Number.isNaN(value)) {
    return "—";
  }
  return `${value.toFixed(1)}%`;
}

export function formatBytes(value?: number | null): string {
  if (value == null || Number.isNaN(value) || value < 0) {
    return "—";
  }
  if (value === 0) {
    return "0 B";
  }
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let current = value;
  let unitIndex = 0;
  while (current >= 1024 && unitIndex < units.length - 1) {
    current /= 1024;
    unitIndex += 1;
  }
  const precision = current >= 10 ? 1 : 2;
  return `${current.toFixed(precision)} ${units[unitIndex]}`;
}

export function releaseStatusLabel(status: number): string {
  return (
    {
      1: "排队中",
      2: "派发中",
      3: "部署中",
      4: "待切流",
      5: "已切流",
      6: "已完成",
      7: "失败",
      8: "已回滚",
      9: "已跳过",
    }[status] ?? `状态 ${status}`
  );
}

export function taskStatusLabel(status: number): string {
  return (
    {
      1: "待执行",
      2: "已派发",
      3: "运行中",
      4: "成功",
      5: "失败",
      6: "超时",
    }[status] ?? `任务 ${status}`
  );
}

export function taskTypeLabel(type: number): string {
  return (
    {
      1: "部署绿槽",
      2: "切换流量",
      3: "回滚",
      4: "清理旧容器",
    }[type] ?? `类型 ${type}`
  );
}

export function taskStepLabel(step: string): string {
  return (
    {
      accepted: "接受任务",
      dispatched: "派发任务",
      "replayed_after_reconnect": "重连后重派",
      recovered_running: "恢复运行",
      image_pulled: "拉取镜像",
      image_pull_failed: "拉取镜像失败",
      container_started: "启动服务",
      startup_grace_started: "启动宽限期",
      health_probe_retry: "健康探测",
      healthy: "健康检查通过",
      "health_check_failed": "健康检查失败",
      cleanup_pruned: "清理容器",
      cleanup_failed: "清理失败",
      traffic_switched: "切换流量",
      task_timed_out: "任务超时",
      managed_container_conflict: "容器冲突",
      proxy_stack_not_ready: "代理栈未就绪",
      noop: "无操作",
    }[step] ?? step
  );
}

export function auditEventLabel(eventType: string, message: string): string {
  if (eventType === "traffic_percent_updated") {
    const match = message.match(/percent=(\d+)/);
    const percent = match ? match[1] : "?";
    return `调整流量至 ${percent}%`;
  }
  if (eventType === "switch_confirmed") {
    return "确认切流 100%";
  }
  if (eventType === "traffic_switched") {
    return "流量切换完成";
  }
  return eventType;
}

export function slotLabel(slot: number): string {
  if (slot === 1) {
    return "蓝槽";
  }
  if (slot === 2) {
    return "绿槽";
  }
  return "未设置";
}

export function releaseStatusTone(status: number, isActive: boolean): "default" | "success" | "danger" | "warning" {
  if (status === 7) {
    return "danger";
  }
  if (status === 6 || status === 8) {
    return "success";
  }
  if (isActive || status === 4) {
    return "warning";
  }
  return "default";
}

export function taskStatusTone(status: number): "default" | "success" | "danger" | "warning" {
  if (status === 4) {
    return "success";
  }
  if (status === 5 || status === 6) {
    return "danger";
  }
  if (status === 2 || status === 3) {
    return "warning";
  }
  return "default";
}

export function formatDuration(ms: number): string {
  if (ms < 0 || Number.isNaN(ms)) {
    return "—";
  }
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) {
    return `${days}d ${hours % 24}h ${minutes % 60}m`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`;
  }
  return `${seconds}s`;
}

export function getTaskDuration(task: {
  startedAt: string | null;
  completedAt: string | null;
  status: number;
}): number | null {
  if (!task.startedAt) {
    return null;
  }
  const start = new Date(task.startedAt).getTime();
  const end = task.completedAt ? new Date(task.completedAt).getTime() : Date.now();
  return end - start;
}
