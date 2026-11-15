import { ActionButton } from "../../../shared/components/ActionButton";
import type { ReleaseRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface ReleaseActionsProps {
  release: ReleaseRecord;
  busy: boolean;
  refreshPending: boolean;
  startPending: boolean;
  skipPending: boolean;
  confirmPending: boolean;
  rollbackPending: boolean;
  retryPending: boolean;
  onStart: () => void;
  onSkip: () => void;
  onConfirm: () => void;
  onRollback: () => void;
  onRetry: () => void;
  onBack: () => void;
  onRefresh: () => void;
}

export function ReleaseActions({
  release,
  busy,
  refreshPending,
  startPending,
  skipPending,
  confirmPending,
  rollbackPending,
  retryPending,
  onStart,
  onSkip,
  onConfirm,
  onRollback,
  onRetry,
  onBack,
  onRefresh,
}: ReleaseActionsProps) {
  return (
    <div className={styles.buttonRow}>
      <ActionButton label="返回发布" onClick={onBack} variant="ghost" />
      <ActionButton
        label="刷新"
        pending={refreshPending}
        onClick={onRefresh}
        variant="ghost"
      />
      {release.verificationUrl && release.status >= 4 ? (
        <a
          className={styles.secondaryButton}
          href={release.verificationUrl}
          target="_blank"
          rel="noopener noreferrer"
        >
          验证目标版本 ↗
        </a>
      ) : null}
      {release.status === 1 || release.status === 9 ? (
        <ActionButton
          label="开始部署"
          pending={startPending}
          variant="primary"
          disabled={busy}
          onClick={onStart}
        />
      ) : null}
      {release.status === 1 ? (
        <ActionButton
          label="跳过"
          pending={skipPending}
          variant="ghost"
          disabled={busy}
          onClick={onSkip}
        />
      ) : null}
      {release.status === 4 ? (
        <>
          <ActionButton
            label="确认切流"
            pending={confirmPending}
            variant="primary"
            disabled={busy}
            onClick={onConfirm}
          />
          <ActionButton
            label="回滚"
            pending={rollbackPending}
            variant="danger"
            disabled={busy}
            onClick={onRollback}
          />
        </>
      ) : null}
      {release.status === 7 ? (
        <ActionButton
          label="重试部署"
          pending={retryPending}
          variant="primary"
          disabled={busy}
          onClick={onRetry}
        />
      ) : null}
    </div>
  );
}
