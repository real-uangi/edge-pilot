import { ActionButton } from "../../../shared/components/ActionButton";
import type { AgentRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface AgentActionsProps {
  agent: AgentRecord;
  deleteBlockedReason: string | null;
  enablePending: boolean;
  disablePending: boolean;
  resetPending: boolean;
  deletePending: boolean;
  credentialVisible: boolean;
  onEnable: () => void;
  onDisable: () => void;
  onResetToken: () => void;
  onDelete: () => void;
  onBack: () => void;
  onRefresh: () => void;
}

export function AgentActions({
  agent,
  deleteBlockedReason,
  enablePending,
  disablePending,
  resetPending,
  deletePending,
  credentialVisible,
  onEnable,
  onDisable,
  onResetToken,
  onDelete,
  onBack,
  onRefresh,
}: AgentActionsProps) {
  const busy = enablePending || disablePending || resetPending || deletePending;
  return (
    <div className={styles.buttonRow}>
      <ActionButton label="返回" onClick={onBack} />
      <ActionButton label="刷新" onClick={onRefresh} />
      {agent.enabled === false ? (
        <ActionButton
          label="启用节点"
          disabled={busy}
          pending={enablePending}
          onClick={onEnable}
        />
      ) : (
        <ActionButton
          label="停用节点"
          disabled={busy}
          pending={disablePending}
          onClick={onDisable}
        />
      )}
      <ActionButton
        label="重置令牌"
        pending={resetPending}
        pendingLabel="重置中"
        variant="ghost"
        disabled={busy || credentialVisible}
        onClick={onResetToken}
      />
      <ActionButton
        label="删除节点"
        pending={deletePending}
        pendingLabel="删除中"
        variant="danger"
        disabled={busy || Boolean(deleteBlockedReason)}
        onClick={onDelete}
      />
    </div>
  );
}
