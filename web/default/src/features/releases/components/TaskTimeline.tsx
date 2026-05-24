import { useEffect, useMemo, useState } from "react";
import {
  formatDateTime,
  formatDuration,
  getTaskDuration,
  taskStatusLabel,
  taskStatusTone,
  taskTypeLabel,
} from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState } from "../../../shared/components/StateBlocks";
import { TaskLogModal } from "./TaskLogModal";
import type { TaskSnapshot } from "../types";
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

function DurationDisplay({ task }: { task: TaskSnapshot }) {
  const [, setTick] = useState(0);

  const isRunning = isTaskRunning(task.status);
  const duration = getTaskDuration(task);
  const durationText = duration != null ? formatDuration(duration) : "—";

  useEffect(() => {
    if (!isRunning || !task.startedAt) {
      return;
    }
    const interval = setInterval(() => {
      setTick((t) => t + 1);
    }, 1000);
    return () => clearInterval(interval);
  }, [isRunning, task.startedAt]);

  return (
    <span
      className={`${styles.pipelineDuration} ${isRunning ? styles.pipelineDurationRunning : ""}`}
    >
      {durationText}
    </span>
  );
}

function PipelineStep({
  task,
  isLast,
  onClick,
}: {
  task: TaskSnapshot;
  isLast: boolean;
  onClick: () => void;
}) {
  return (
    <div className={styles.pipelineStep}>
      <div className={styles.pipelineIndicator}>
        <div className={`${styles.pipelineNode} ${getNodeClass(task.status)}`} />
        {!isLast && (
          <div className={`${styles.pipelineConnector} ${getConnectorClass(task.status)}`} />
        )}
      </div>
      <div className={styles.pipelineCard} onClick={onClick} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onClick(); } }}>
        <div className={styles.pipelineCardHeader}>
          <h3 className={styles.pipelineCardTitle}>{taskTypeLabel(task.type)}</h3>
          <StatusPill label={taskStatusLabel(task.status)} tone={taskStatusTone(task.status)} />
        </div>
        <div className={styles.pipelineCardMeta}>
          <div className={styles.pipelineCardMetaItem}>
            <span className={styles.pipelineCardMetaLabel}>派发</span>
            <span>{formatDateTime(task.dispatchedAt)}</span>
          </div>
          <div className={styles.pipelineCardMetaItem}>
            <span className={styles.pipelineCardMetaLabel}>开始</span>
            <span>{formatDateTime(task.startedAt)}</span>
          </div>
          <div className={styles.pipelineCardMetaItem}>
            <span className={styles.pipelineCardMetaLabel}>完成</span>
            <span>{formatDateTime(task.completedAt)}</span>
          </div>
          <div className={styles.pipelineCardMetaItem}>
            <span className={styles.pipelineCardMetaLabel}>耗时</span>
            <DurationDisplay task={task} />
          </div>
        </div>
      </div>
    </div>
  );
}

export function TaskTimeline({ tasks }: TaskTimelineProps) {
  const [selectedTask, setSelectedTask] = useState<TaskSnapshot | null>(null);

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
      {!tasks.length ? (
        <EmptyState title="暂无任务" message="该发布单还未生成执行任务。" />
      ) : (
        <div className={styles.pipelineContainer}>
          {tasks.map((task, index) => (
            <PipelineStep
              key={task.id}
              task={task}
              isLast={index === tasks.length - 1}
              onClick={() => setSelectedTask(task)}
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
