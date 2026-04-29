import { ActionButton } from "../../../shared/components/ActionButton";
import type { AgentRecord, AgentCredentialRecord } from "../types";

interface AgentActionsProps {
  agent: AgentRecord;
  deleteBlockedReason: string | null;
  enablePending: boolean;
  disablePending: boolean;
  resetPending: boolean;
  deletePending: boolean;
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
  onEnable,
  onDisable,
  onResetToken,
  onDelete,
  onBack,
  onRefresh,
}: AgentActionsProps) {
  return (
    <div className="buttonRow">
      <ActionButton label="返回" onClick={onBack} />
      <ActionButton label="刷新" onClick={onRefresh} />
      <ActionButton label="启用" variant="ghost" pending={enablePending} onClick={onEnable} />
      <ActionButton label="停用" variant="danger" pending={disablePending} onClick={onDisable} />
      <ActionButton
        label="重置令牌"
        pending={resetPending}
        pendingLabel="重置中"
        variant="primary"
        onClick={onResetToken}
      />
      <ActionButton
        label="删除节点"
        pending={deletePending}
        pendingLabel="删除中"
        variant="danger"
        disabled={Boolean(deleteBlockedReason)}
        onClick={onDelete}
      />
    </div>
  );
}
