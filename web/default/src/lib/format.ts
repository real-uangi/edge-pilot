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

export function boolLabel(value?: boolean | null, trueText = "Yes", falseText = "No"): string {
  if (value == null) {
    return "Unknown";
  }
  return value ? trueText : falseText;
}

export function releaseStatusLabel(status: number): string {
  return (
    {
      1: "Queued",
      2: "Dispatching",
      3: "Deploying",
      4: "Ready To Switch",
      5: "Switched",
      6: "Completed",
      7: "Failed",
      8: "Rolled Back",
      9: "Skipped",
    }[status] ?? `Status ${status}`
  );
}

export function taskStatusLabel(status: number): string {
  return (
    {
      1: "Pending",
      2: "Dispatched",
      3: "Running",
      4: "Succeeded",
      5: "Failed",
      6: "Timed Out",
    }[status] ?? `Task ${status}`
  );
}

export function taskTypeLabel(type: number): string {
  return (
    {
      1: "Deploy Green",
      2: "Switch Traffic",
      3: "Rollback",
      4: "Cleanup Old",
    }[type] ?? `Type ${type}`
  );
}

export function slotLabel(slot: number): string {
  if (slot === 1) {
    return "Blue";
  }
  if (slot === 2) {
    return "Green";
  }
  return "None";
}
