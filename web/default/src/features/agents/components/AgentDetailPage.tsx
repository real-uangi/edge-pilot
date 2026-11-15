import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { useDialog } from "../../../shared/components/DialogProvider";
import { CredentialResult } from "../../../shared/components/CredentialResult";
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
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
  const [issuedCredential, setIssuedCredential] =
    useState<AgentCredentialRecord | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const currentAgentId = useRef(id);
  useEffect(() => {
    currentAgentId.current = id;
    setIssuedCredential(null);
    setActionError(null);
    return () => {
      currentAgentId.current = undefined;
    };
  }, [id]);

  const agentQuery = useQuery({
    queryKey: ["agent", id],
    queryFn: () => agentsApi.get(id!),
    enabled: Boolean(id),
    refetchInterval: 10000,
  });
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: servicesApi.list,
    refetchInterval: 10000,
  });

  const refreshQueries = async (targetId: string) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["agent", targetId] }),
      queryClient.invalidateQueries({ queryKey: ["agents"] }),
      queryClient.invalidateQueries({ queryKey: ["overview"] }),
      queryClient.invalidateQueries({ queryKey: ["services"] }),
    ]);
  };

  const enableMutation = useMutation({
    mutationFn: (targetId: string) => agentsApi.enable(targetId),
    onSuccess: async (_, targetId) => {
      if (currentAgentId.current === targetId) setActionError(null);
      await refreshQueries(targetId);
    },
    onError: (error, targetId) => {
      if (currentAgentId.current === targetId)
        setActionError(getErrorMessage(error));
    },
  });
  const disableMutation = useMutation({
    mutationFn: (targetId: string) => agentsApi.disable(targetId),
    onSuccess: async (_, targetId) => {
      if (currentAgentId.current === targetId) setActionError(null);
      await refreshQueries(targetId);
    },
    onError: (error, targetId) => {
      if (currentAgentId.current === targetId)
        setActionError(getErrorMessage(error));
    },
  });
  const resetMutation = useMutation({
    mutationFn: async (targetId: string) => {
      const output = await agentsApi.resetToken(targetId);
      if (currentAgentId.current === targetId) setIssuedCredential(output);
    },
    onSuccess: async (_, targetId) => {
      if (currentAgentId.current === targetId) setActionError(null);
      await refreshQueries(targetId);
    },
    onError: (error, targetId) => {
      if (currentAgentId.current === targetId)
        setActionError(getErrorMessage(error));
    },
  });
  const deleteMutation = useMutation({
    mutationFn: (targetId: string) => agentsApi.delete(targetId),
    onSuccess: async (_, targetId) => {
      if (currentAgentId.current === targetId) setActionError(null);
      await refreshQueries(targetId);
      if (currentAgentId.current === targetId)
        navigate("/agents", { replace: true });
    },
    onError: (error, targetId) => {
      if (currentAgentId.current === targetId)
        setActionError(getErrorMessage(error));
    },
  });

  if (agentQuery.isPending) {
    return (
      <div className={styles.page}>
        <LoadingState
          title="正在加载节点详情"
          message="正在同步节点基础信息。"
        />
      </div>
    );
  }

  if (agentQuery.isError && !agentQuery.data) {
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
  const boundServiceCount = (servicesQuery.data ?? []).filter(
    (service) => service.agentId === agent.id,
  ).length;
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
    const targetId = agent.id;
    const confirmed = await dialog.confirm({
      title: "删除节点",
      message: `确认删除节点「${agent.hostname || agent.ip || agent.id}」？此操作无法撤销。`,
      confirmText: "确认删除",
      cancelText: "取消",
      danger: true,
    });
    if (confirmed) {
      deleteMutation.mutate(targetId);
    }
  };

  return (
    <div className={styles.page}>
      <section className={styles.pageHeader}>
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
          credentialVisible={Boolean(issuedCredential)}
          onEnable={() => enableMutation.mutate(agent.id)}
          onDisable={() => disableMutation.mutate(agent.id)}
          onResetToken={async () => {
            const targetId = agent.id;
            if (
              await dialog.confirm({
                title: "重置节点令牌",
                message: `重置「${agent.hostname || agent.ip || agent.id}」的令牌后，旧令牌立即失效。确认重置？`,
                confirmText: "重置令牌",
                danger: true,
              })
            )
              resetMutation.mutate(targetId);
          }}
          onDelete={() => void handleDelete()}
          onBack={() => navigate("/agents")}
          onRefresh={() =>
            void Promise.all([agentQuery.refetch(), servicesQuery.refetch()])
          }
        />
      </section>

      <QueryRefreshNotice
        query={agentQuery}
        onRetry={() => void agentQuery.refetch()}
      />

      {actionError ? <InlineNotice message={actionError} tone="error" /> : null}
      {deleteBlockedReason ? (
        <InlineNotice message={deleteBlockedReason} tone="info" />
      ) : null}

      {issuedCredential ? (
        <CredentialResult
          id={issuedCredential.id}
          token={issuedCredential.token}
          onClose={() => setIssuedCredential(null)}
        />
      ) : null}

      <AgentInfo
        agent={agent}
        services={servicesQuery.data ?? []}
        servicesPending={servicesQuery.isPending}
        servicesError={servicesQuery.isError && !servicesQuery.data}
      />

      <HAProxyConfig agentId={agent.id} agentOnline={agent.online} />
    </div>
  );
}
