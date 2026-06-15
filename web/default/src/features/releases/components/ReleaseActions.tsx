import { ActionButton } from "../../../shared/components/ActionButton";
import type { ReleaseRecord } from "../types";

interface ReleaseActionsProps {
  release: ReleaseRecord;
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
  const canAdjustTraffic = release.status === 4;

  return (
    <div className="buttonRow">
      <ActionButton label="返回" onClick={onBack} />
      <ActionButton label="刷新" onClick={onRefresh} />
      <ActionButton
        label="验证目标版本"
        disabled={!release.verificationUrl || release.status < 4}
        onClick={() => {
          if (!release.verificationUrl) {
            return;
          }
          window.open(release.verificationUrl, "_blank", "noopener,noreferrer");
        }}
      />
      <ActionButton
        label="开始"
        pending={startPending}
        variant="primary"
        disabled={release.status !== 1 && release.status !== 9}
        onClick={onStart}
      />
      <ActionButton
        label="跳过"
        pending={skipPending}
        variant="ghost"
        disabled={release.status !== 1}
        onClick={onSkip}
      />
      <ActionButton
        label="确认切流"
        pending={confirmPending}
        variant="primary"
        disabled={release.status !== 4}
        onClick={onConfirm}
      />
      <ActionButton
        label="回滚"
        pending={rollbackPending}
        variant="danger"
        disabled={!canAdjustTraffic}
        onClick={onRollback}
      />
      <ActionButton
        label="重试"
        pending={retryPending}
        variant="ghost"
        disabled={release.status !== 7}
        onClick={onRetry}
      />
    </div>
  );
}
