import { useEffect, useMemo, useState } from "react";
import {
  formatDateTime,
  formatDuration,
  getTaskDuration,
  taskStatusLabel,
  taskStatusTone,
  taskStepLabel,
  taskTypeLabel,
  auditEventLabel,
} from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState } from "../../../shared/components/StateBlocks";
import { TaskLogModal } from "./TaskLogModal";
import type { TaskSnapshot, TaskAttempt, AuditLog } from "../types";
import styles from "../../../styles/admin.module.css";

type ReleaseTaskLogInfo = {
  lastStep: string;
  dockerHealth: string;
  cleanupCompleted: boolean | null;
  lastError: string;
  failureLogs: string;
};

interface TaskTimelineProps {
  tasks: TaskSnapshot[];
  audits: AuditLog[];
}

function isTaskRunning(status: number): boolean {
  return status === 2 || status === 3;
}

function isTaskFinished(status: number): boolean {
  return status === 4 || status === 5 || status === 6;
}

function isTaskSuccess(status: number): boolean {
  return status === 4;
}

function isTaskFailed(status: number): boolean {
  return status === 5 || status === 6;
}

function getNodeClass(status: number): string {
  if (isTaskRunning(status)) {
    return styles.pipelineNodeRunning;
  }
  if (isTaskSuccess(status)) {
    return styles.pipelineNodeSuccess;
  }
  if (isTaskFailed(status)) {
    return styles.pipelineNodeFailed;
  }
  if (status === 1) {
    return "";
  }
  return styles.pipelineNodeWarning;
}

function getConnectorClass(status: number): string {
  if (isTaskSuccess(status)) {
    return styles.pipelineConnectorDone;
  }
  if (isTaskFailed(status)) {
    return styles.pipelineConnectorFailed;
  }
  return "";
}

function DurationDisplay({ startedAt, completedAt, isRunning }: { startedAt: string | null; completedAt: string | null; isRunning: boolean }) {
  const [, setTick] = useState(0);

  const duration = useMemo(() => {
    if (!startedAt) return null;
    const start = new Date(startedAt).getTime();
    const end = completedAt ? new Date(completedAt).getTime() : Date.now();
    return end - start;
  }, [startedAt, completedAt]);

  const durationText = duration != null ? formatDuration(duration) : "—";

  useEffect(() => {
    if (!isRunning || !startedAt) {
      return;
    }
    const interval = setInterval(() => {
      setTick((t) => t + 1);
    }, 1000);
    return () => clearInterval(interval);
  }, [isRunning, startedAt]);

  return (
    <span
      className={`${styles.pipelineDuration} ${isRunning ? styles.pipelineDurationRunning : ""}`}
    >
      {durationText}
    </span>
  );
}

interface PipelineNode {
  id: string;
  type: "task-step" | "audit";
  title: string;
  status: number;
  startedAt: string | null;
  completedAt: string | null;
  isRunning: boolean;
  meta?: Record<string, string>;
  taskId?: string;
}

function buildPipelineNodes(tasks: TaskSnapshot[], audits: AuditLog[]): PipelineNode[] {
  const nodes: PipelineNode[] = [];

  for (const task of tasks) {
    if (task.attempts && task.attempts.length > 0) {
      for (const attempt of task.attempts) {
        nodes.push({
          id: `attempt-${attempt.id}`,
          type: "task-step",
          title: taskStepLabel(attempt.message) || attempt.message,
          status: attempt.status,
          startedAt: attempt.startedAt,
          completedAt: attempt.completedAt,
          isRunning: isTaskRunning(attempt.status),
          taskId: task.id,
        });
      }
    } else {
      // Fallback: if no attempts, show the task itself as a single node
      nodes.push({
        id: `task-${task.id}`,
        type: "task-step",
        title: taskTypeLabel(task.type),
        status: task.status,
        startedAt: task.startedAt,
        completedAt: task.completedAt,
        isRunning: isTaskRunning(task.status),
        taskId: task.id,
      });
    }
  }

  for (const audit of audits) {
    const trafficEvents = ["traffic_percent_updated", "switch_confirmed", "traffic_switched"];
    if (trafficEvents.includes(audit.eventType)) {
      nodes.push({
        id: `audit-${audit.id}`,
        type: "audit",
        title: auditEventLabel(audit.eventType, audit.message),
        status: 4, // success
        startedAt: audit.createdAt,
        completedAt: audit.createdAt,
        isRunning: false,
      });
    }
  }

  nodes.sort((a, b) => {
    const aTime = a.startedAt ? new Date(a.startedAt).getTime() : 0;
    const bTime = b.startedAt ? new Date(b.startedAt).getTime() : 0;
    return aTime - bTime;
  });

  return nodes;
}

function PipelineNodeComponent({
  node,
  isLast,
  onClick,
}: {
  node: PipelineNode;
  isLast: boolean;
  onClick: () => void;
}) {
  return (
    <div className={styles.pipelineStep}>
      <div className={styles.pipelineIndicator}>
        <div className={`${styles.pipelineNode} ${getNodeClass(node.status)}`} />
        {!isLast && (
          <div className={`${styles.pipelineConnector} ${getConnectorClass(node.status)}`} />
        )}
      </div>
      <div className={styles.pipelineCard} onClick={onClick} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onClick(); } }}>
        <div className={styles.pipelineCardHeader}>
          <h3 className={styles.pipelineCardTitle}>{node.title}</h3>
          {node.type === "task-step" && (
            <StatusPill label={taskStatusLabel(node.status)} tone={taskStatusTone(node.status)} />
          )}
          {node.type === "audit" && (
            <span className={styles.pipelineAuditBadge}>操作记录</span>
          )}
        </div>
        <div className={styles.pipelineCardMeta}>
          <div className={styles.pipelineCardMetaItem}>
            <span className={styles.pipelineCardMetaLabel}>时间</span>
            <span>{formatDateTime(node.startedAt)}</span>
          </div>
          <div className={styles.pipelineCardMetaItem}>
            <span className={styles.pipelineCardMetaLabel}>耗时</span>
            <DurationDisplay startedAt={node.startedAt} completedAt={node.completedAt} isRunning={node.isRunning} />
          </div>
        </div>
      </div>
    </div>
  );
}

export function TaskTimeline({ tasks, audits }: TaskTimelineProps) {
  const [selectedTask, setSelectedTask] = useState<TaskSnapshot | null>(null);

  const nodes = useMemo(() => buildPipelineNodes(tasks, audits), [tasks, audits]);

  const logInfo = useMemo<ReleaseTaskLogInfo | null>(() => {
    if (!selectedTask) {
      return null;
    }
    return {
      lastStep: selectedTask.lastStep,
      dockerHealth: selectedTask.dockerHealth,
      cleanupCompleted: selectedTask.cleanupCompleted,
      lastError: selectedTask.lastError,
      failureLogs: selectedTask.failureLogs,
    };
  }, [selectedTask]);

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>任务流水线</h2>
        </div>
      </div>
      {!nodes.length ? (
        <EmptyState title="暂无任务" message="该发布单还未生成执行任务。" />
      ) : (
        <div className={styles.pipelineContainer}>
          {nodes.map((node, index) => (
            <PipelineNodeComponent
              key={node.id}
              node={node}
              isLast={index === nodes.length - 1}
              onClick={() => {
                if (node.type === "task-step" && node.taskId) {
                  const task = tasks.find((t) => t.id === node.taskId);
                  if (task) setSelectedTask(task);
                }
              }}
            />
          ))}
        </div>
      )}

      {selectedTask && logInfo && (
        <TaskLogModal
          task={logInfo}
          taskName={taskTypeLabel(selectedTask.type)}
          onClose={() => setSelectedTask(null)}
        />
      )}
    </section>
  );
}
