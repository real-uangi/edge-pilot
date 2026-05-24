import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ActionButton } from "../../../shared/components/ActionButton";
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

function cleanupLabel(value: boolean | null) {
  if (value == null) {
    return "—";
  }
  return value ? "已清理" : "未清理";
}

export function TaskLogModal({ task, taskName, onClose }: TaskLogModalProps) {
  const logRef = useRef<HTMLPreElement | null>(null);
  const [isClosing, setIsClosing] = useState(false);

  useEffect(() => {
    if (logRef.current && task.failureLogs) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [task.failureLogs]);

  useEffect(() => {
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, []);

  const handleClose = () => {
    setIsClosing(true);
    setTimeout(onClose, 200);
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      handleClose();
    }
  };

  return createPortal(
    <div
      className={`${styles.modalBackdrop} ${isClosing ? styles.modalBackdropClosing : ""}`}
      onClick={handleBackdropClick}
      onKeyDown={handleKeyDown}
      role="dialog"
      aria-modal="true"
      aria-labelledby="task-log-title"
      tabIndex={-1}
    >
      <div className={`${styles.modalContent} ${isClosing ? styles.modalContentClosing : ""}`}>
        <div className={styles.modalHeader}>
          <h2 className={styles.modalTitle} id="task-log-title">
            {taskName}
          </h2>
          <button className={styles.modalClose} onClick={handleClose} aria-label="关闭">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M12 4L4 12M4 4L12 12" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        <div className={styles.modalBody}>
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
        </div>

        <div className={styles.modalActions}>
          <ActionButton label="关闭" onClick={handleClose} variant="secondary" />
        </div>
      </div>
    </div>,
    document.body,
  );
}
