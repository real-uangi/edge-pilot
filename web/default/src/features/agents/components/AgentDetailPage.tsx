import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { useDialog } from "../../../shared/components/DialogProvider";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../../../shared/components/StateBlocks";
import { agentsApi } from "../api";
import { servicesApi } from "../../services/api";
import type { AgentCredentialRecord } from "../types";
import { AgentInfo } from "./AgentInfo";
import { AgentActions } from "./AgentActions";
import { HAProxyConfig } from "./HAProxyConfig";
import styles from "../../../styles/admin.module.css";

export function AgentDetailPage() {
  const dialog = useDialog();
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [issuedCredential, setIssuedCredential] = useState<AgentCredentialRecord | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const agentQuery = useQuery({
    queryKey: ["agent", id],
    queryFn: () => agentsApi.get(id!),
    enabled: Boolean(id),
    refetchInterval: 10000,
  });
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: servicesApi.list,
  });

  const refreshQueries = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["agent", id] }),
      queryClient.invalidateQueries({ queryKey: ["agents"] }),
      queryClient.invalidateQueries({ queryKey: ["overview"] }),
      queryClient.invalidateQueries({ queryKey: ["services"] }),
    ]);
  };

  const enableMutation = useMutation({
    mutationFn: () => agentsApi.enable(id!),
    onSuccess: async () => {
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const disableMutation = useMutation({
    mutationFn: () => agentsApi.disable(id!),
    onSuccess: async () => {
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const resetMutation = useMutation({
    mutationFn: () => agentsApi.resetToken(id!),
    onSuccess: async (output) => {
      setIssuedCredential(output);
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const deleteMutation = useMutation({
    mutationFn: () => agentsApi.delete(id!),
    onSuccess: async () => {
      setActionError(null);
      await refreshQueries();
      navigate("/agents", { replace: true });
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });

  if (agentQuery.isPending) {
    return (
      <div className={styles.page}>
        <LoadingState title="正在加载节点详情" message="正在同步节点基础信息。" />
      </div>
    );
  }

  if (agentQuery.isError) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="节点详情加载失败"
          message={getErrorMessage(agentQuery.error)}
          onRetry={() => agentQuery.refetch()}
        />
      </div>
    );
  }

  if (!agentQuery.data) {
    return (
      <div className={styles.page}>
        <EmptyState title="节点不存在" message="请返回节点列表后重新选择。" />
      </div>
    );
  }

  const agent = agentQuery.data;
  const boundServiceCount = (servicesQuery.data ?? []).filter((service) => service.agentId === agent.id).length;
  const deleteBlockedReason = agent.online
    ? "在线节点不可删除"
    : servicesQuery.isPending
      ? "正在检查节点绑定状态"
      : servicesQuery.isError
        ? "节点绑定状态加载失败"
        : boundServiceCount > 0
          ? `该节点仍绑定 ${boundServiceCount} 个服务`
          : null;

  const handleDelete = async () => {
    const confirmed = await dialog.confirm({
      title: "删除节点",
      message: "确认删除这个节点？",
      confirmText: "确认删除",
      cancelText: "取消",
      danger: true,
    });
    if (confirmed) {
      deleteMutation.mutate();
    }
  };

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>节点详情</h1>
          <p className={styles.sectionCopy}>
            <AgentLabel id={agent.id} hostname={agent.hostname} ip={agent.ip} />
          </p>
        </div>
        <AgentActions
          agent={agent}
          deleteBlockedReason={deleteBlockedReason}
          enablePending={enableMutation.isPending}
          disablePending={disableMutation.isPending}
          resetPending={resetMutation.isPending}
          deletePending={deleteMutation.isPending}
          onEnable={() => enableMutation.mutate()}
          onDisable={() => disableMutation.mutate()}
          onResetToken={() => resetMutation.mutate()}
          onDelete={() => void handleDelete()}
          onBack={() => navigate("/agents")}
          onRefresh={() => agentQuery.refetch()}
        />
      </section>

      {actionError ? <InlineNotice message={actionError} tone="error" /> : null}
      {deleteBlockedReason ? <InlineNotice message={deleteBlockedReason} tone="info" /> : null}

      {issuedCredential ? (
        <section className={styles.credentialCard}>
          <span className={styles.eyebrow}>一次性令牌</span>
          <div>
            <strong>ID</strong>
            <div className={styles.credentialValue}>{issuedCredential.id}</div>
          </div>
          <div>
            <strong>令牌</strong>
            <div className={styles.credentialValue}>{issuedCredential.token}</div>
          </div>
        </section>
      ) : null}

      <AgentInfo agent={agent} />

      <HAProxyConfig agentId={agent.id} agentOnline={agent.online} />
    </div>
  );
}
