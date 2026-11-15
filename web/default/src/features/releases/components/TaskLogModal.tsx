import { useEffect, useRef } from "react";
import { Modal } from "../../../shared/components/Modal";
import styles from "../../../styles/admin.module.css";

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
  onClose: () => void;
}

export function TaskLogModal({ task, taskName, onClose }: TaskLogModalProps) {
  const logRef = useRef<HTMLPreElement | null>(null);
  useEffect(() => {
    if (logRef.current && task.failureLogs)
      logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [task.failureLogs]);

  return (
    <Modal title={`${taskName} · 任务日志`} onClose={onClose} variant="wide">
      <div className={styles.logMeta}>
        <span>阶段：{task.lastStep || "—"}</span>
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
    </Modal>
  );
}
