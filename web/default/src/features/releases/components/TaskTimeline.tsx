import { useRef, useEffect } from "react";
import { formatDateTime, taskStatusLabel, taskStatusTone, taskTypeLabel } from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState } from "../../../shared/components/StateBlocks";
import type { TaskSnapshot } from "../types";
import styles from "../../../styles/admin.module.css";

function cleanupLabel(value: boolean | null) {
  if (value == null) {
    return "—";
  }
  return value ? "已清理" : "未清理";
}

type ReleaseTaskLogInfo = {
  lastStep: string;
  dockerHealth: string;
  cleanupCompleted: boolean | null;
  lastError: string;
  failureLogs: string;
};

function TaskLogDetails({ task }: { task: ReleaseTaskLogInfo }) {
  const detailsRef = useRef<HTMLDetailsElement | null>(null);
  const logRef = useRef<HTMLPreElement | null>(null);

  const scrollLogToBottom = () => {
    if (!logRef.current) {
      return;
    }
    logRef.current.scrollTop = logRef.current.scrollHeight;
  };

  useEffect(() => {
    if (detailsRef.current?.open) {
      scrollLogToBottom();
    }
  }, [task.failureLogs]);

  return (
    <details
      className={styles.logCard}
      ref={detailsRef}
      onToggle={() => {
        if (detailsRef.current?.open) {
          scrollLogToBottom();
        }
      }}
    >
      <summary className={styles.logSummary}>查看详情和日志</summary>
      <div className={styles.logMeta}>
        <span>阶段：{task.lastStep || "—"}</span>
        <span>Docker 状态：{task.dockerHealth || "—"}</span>
        <span>清理：{cleanupLabel(task.cleanupCompleted)}</span>
        <span>错误：{task.lastError || "—"}</span>
      </div>
      {task.failureLogs ? (
        <pre className={styles.logBlock} ref={logRef}>
          {task.failureLogs}
        </pre>
      ) : (
        <div className={styles.logEmpty}>暂无失败日志</div>
      )}
    </details>
  );
}

interface TaskTimelineProps {
  tasks: TaskSnapshot[];
}

export function TaskTimeline({ tasks }: TaskTimelineProps) {
  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>任务时间线</h2>
        </div>
      </div>
      {!tasks.length ? (
        <EmptyState title="暂无任务" message="该发布单还未生成执行任务。" />
      ) : (
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>任务</th>
                <th>状态</th>
                <th>派发时间</th>
                <th>开始时间</th>
                <th>完成时间</th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((task) => (
                [
                  <tr key={task.id}>
                    <td>{taskTypeLabel(task.type)}</td>
                    <td>
                      <StatusPill label={taskStatusLabel(task.status)} tone={taskStatusTone(task.status)} />
                    </td>
                    <td>{formatDateTime(task.dispatchedAt)}</td>
                    <td>{formatDateTime(task.startedAt)}</td>
                    <td>{formatDateTime(task.completedAt)}</td>
                  </tr>,
                  <tr className={styles.timelineDetailRow} key={task.id + "-details"}>
                    <td className={styles.timelineDetailCell} colSpan={5}>
                      <TaskLogDetails task={task} />
                    </td>
                  </tr>,
                ]
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
