import { useEffect, useMemo, useState } from "react";
import {
  formatDateTime,
  formatDuration,
  taskStatusLabel,
  taskStatusTone,
  taskStepLabel,
  taskTypeLabel,
  auditEventLabel,
} from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState } from "../../../shared/components/StateBlocks";
import { TaskLogModal } from "./TaskLogModal";
import type { TaskSnapshot, AuditLog } from "../types";
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

function DurationDisplay({
  startedAt,
  completedAt,
  isRunning,
}: {
  startedAt: string | null;
  completedAt: string | null;
  isRunning: boolean;
}) {
  const [, setTick] = useState(0);

  const duration = startedAt
    ? (completedAt ? new Date(completedAt).getTime() : Date.now()) -
      new Date(startedAt).getTime()
    : null;

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

function buildPipelineNodes(
  tasks: TaskSnapshot[],
  audits: AuditLog[],
): PipelineNode[] {
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
    const trafficEvents = [
      "traffic_percent_updated",
      "switch_confirmed",
      "traffic_switched",
    ];
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
  const content = (
    <>
      <span className={styles.pipelineCardHeader}>
        <strong className={styles.pipelineCardTitle}>{node.title}</strong>
        {node.type === "task-step" ? (
          <StatusPill
            label={taskStatusLabel(node.status)}
            tone={taskStatusTone(node.status)}
          />
        ) : (
          <span className={styles.pipelineAuditBadge}>操作记录</span>
        )}
      </span>
      <span className={styles.pipelineCardMeta}>
        <span className={styles.pipelineCardMetaItem}>
          <span className={styles.pipelineCardMetaLabel}>时间</span>
          <span>{formatDateTime(node.startedAt)}</span>
        </span>
        <span className={styles.pipelineCardMetaItem}>
          <span className={styles.pipelineCardMetaLabel}>耗时</span>
          <DurationDisplay
            startedAt={node.startedAt}
            completedAt={node.completedAt}
            isRunning={node.isRunning}
          />
        </span>
      </span>
    </>
  );
  return (
    <div className={styles.pipelineStep}>
      <div className={styles.pipelineIndicator} aria-hidden="true">
        <div
          className={`${styles.pipelineNode} ${getNodeClass(node.status)}`}
        />
        {!isLast ? (
          <div
            className={`${styles.pipelineConnector} ${getConnectorClass(node.status)}`}
          />
        ) : null}
      </div>
      {node.type === "task-step" && node.taskId ? (
        <button
          type="button"
          className={`${styles.pipelineCard} ${styles.pipelineTaskButton}`}
          onClick={onClick}
          aria-label={`查看${node.title}的任务日志`}
        >
          {content}
        </button>
      ) : (
        <div className={`${styles.pipelineCard} ${styles.pipelineAuditCard}`}>
          {content}
        </div>
      )}
    </div>
  );
}

export function TaskTimeline({ tasks, audits }: TaskTimelineProps) {
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const selectedTask = tasks.find((task) => task.id === selectedTaskId) ?? null;

  const nodes = useMemo(
    () => buildPipelineNodes(tasks, audits),
    [tasks, audits],
  );

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
          <h2 className={styles.sectionSubtitle}>任务流水线</h2>
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
                  if (task) setSelectedTaskId(task.id);
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
          onClose={() => setSelectedTaskId(null)}
        />
      )}
    </section>
  );
}
