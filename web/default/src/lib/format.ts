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

export function boolLabel(value?: boolean | null, trueText = "是", falseText = "否"): string {
  if (value == null) {
    return "未知";
  }
  return value ? trueText : falseText;
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
