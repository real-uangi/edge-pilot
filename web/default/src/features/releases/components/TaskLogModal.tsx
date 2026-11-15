import { useEffect, useRef } from "react";
import { Modal } from "../../../shared/components/Modal";
import {
  auditEventLabel,
  formatDateTime,
  taskStepLabel,
} from "../../../shared/lib/format";
import type { PipelineEvent } from "../types";
import styles from "../../../styles/admin.module.css";
import pipelineStyles from "./PipelineTable.module.css";

type TaskLogInfo = {
  lastStep: string;
  dockerHealth: string;
  cleanupCompleted: boolean | null;
  lastError: string;
  failureLogs: string;
};

interface TaskLogModalProps {
  task: TaskLogInfo;
  taskName: string;
  events?: PipelineEvent[];
  onClose: () => void;
}

export function TaskLogModal({ task, taskName, events, onClose }: TaskLogModalProps) {
  const logRef = useRef<HTMLPreElement | null>(null);
  useEffect(() => {
    if (logRef.current && task.failureLogs)
      logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [task.failureLogs]);

  return (
    <Modal title={`${taskName} · 任务日志`} onClose={onClose} variant="wide">
      {events && events.length > 0 ? (
        <section className={pipelineStyles.eventsSection}>
          <h3 className={pipelineStyles.detailTitle}>阶段记录</h3>
          <ol className={pipelineStyles.eventsList}>
            {events.map((event) => (
              <li key={`${event.kind}-${event.id}`} className={pipelineStyles.event}>
                <time dateTime={event.at} className={pipelineStyles.eventAt}>
                  {formatDateTime(event.at)}
                </time>
                <span className={pipelineStyles.eventTitle}>
                  {event.kind === "audit"
                    ? auditEventLabel(event.code, event.message)
                    : taskStepLabel(event.code)}
                </span>
                {event.message && event.message !== event.code ? (
                  <p className={pipelineStyles.eventMessage}>{event.message}</p>
                ) : null}
              </li>
            ))}
          </ol>
        </section>
      ) : null}
      <section className={pipelineStyles.logSection}>
        <h3 className={pipelineStyles.detailTitle}>任务日志</h3>
        <div className={styles.logMeta}>
          <span>最后步骤：{taskStepLabel(task.lastStep) || "—"}</span>
          <span>Docker 状态：{task.dockerHealth || "—"}</span>
          <span>
            清理：
            {task.cleanupCompleted == null
              ? "—"
              : task.cleanupCompleted
                ? "已清理"
                : "未清理"}
          </span>
        </div>
        {task.lastError ? (
          <p className={styles.inlineError}>{task.lastError}</p>
        ) : null}
        {task.failureLogs ? (
          <pre
            className={styles.logBlock}
            ref={logRef}
            tabIndex={0}
            aria-label="失败日志"
          >
            {task.failureLogs}
          </pre>
        ) : (
          <p className={styles.logEmpty}>暂无失败日志</p>
        )}
      </section>
    </Modal>
  );
}
