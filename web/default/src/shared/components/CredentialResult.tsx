import { useState } from "react";
import { Modal } from "./Modal";
import { ActionButton } from "./ActionButton";
import { InlineNotice } from "./StateBlocks";
import styles from "../../styles/admin.module.css";

export function CredentialResult({
  id,
  token,
  onClose,
}: {
  id: string;
  token: string;
  onClose: () => void;
}) {
  const [feedback, setFeedback] = useState<{
    message: string;
    error: boolean;
  } | null>(null);
  const copy = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setFeedback({ message: `${label}已复制`, error: false });
    } catch {
      setFeedback({ message: "复制失败，请选中文本手动复制。", error: true });
    }
  };
  return (
    <Modal title="保存访问凭据" onClose={onClose}>
      <div className={styles.page}>
        <InlineNotice
          message="令牌仅显示一次，关闭前请妥善保存。"
          tone="info"
        />
        <label className={styles.field}>
          <span className={styles.label}>ID</span>
          <input
            className={`${styles.input} ${styles.mono}`}
            readOnly
            value={id}
            onFocus={(event) => event.currentTarget.select()}
          />
        </label>
        <div className={styles.buttonRow}>
          <ActionButton label="复制 ID" onClick={() => void copy(id, "ID")} />
        </div>
        <label className={styles.field}>
          <span className={styles.label}>Token</span>
          <textarea
            className={`${styles.textarea} ${styles.mono}`}
            readOnly
            rows={3}
            value={token}
            onFocus={(event) => event.currentTarget.select()}
          />
        </label>
        <div className={styles.buttonRow}>
          <ActionButton
            label="复制 Token"
            onClick={() => void copy(token, "Token")}
          />
        </div>
        {feedback ? (
          <InlineNotice
            message={feedback.message}
            tone={feedback.error ? "error" : "info"}
          />
        ) : null}
        <div className={styles.buttonRow}>
          <ActionButton
            label="已保存，关闭"
            variant="primary"
            onClick={onClose}
          />
        </div>
      </div>
    </Modal>
  );
}
