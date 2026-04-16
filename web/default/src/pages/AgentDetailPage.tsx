import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { api, getErrorMessage, type AgentCredentialRecord } from "../lib/api";
import { formatDateTime, boolLabel } from "../lib/format";
import { ActionButton } from "../components/ActionButton";
import { AgentLabel } from "../components/AgentLabel";
import { StatusPill } from "../components/StatusPill";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../components/StateBlocks";
import styles from "../styles/admin.module.css";

export function AgentDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [issuedCredential, setIssuedCredential] = useState<AgentCredentialRecord | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const agentQuery = useQuery({
    queryKey: ["agent", id],
    queryFn: () => api.getAgent(id!),
    enabled: Boolean(id),
    refetchInterval: 10000,
  });
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: api.listServices,
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
    mutationFn: () => api.enableAgent(id!),
    onSuccess: async () => {
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const disableMutation = useMutation({
    mutationFn: () => api.disableAgent(id!),
    onSuccess: async () => {
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const resetMutation = useMutation({
    mutationFn: () => api.resetAgentToken(id!),
    onSuccess: async (output) => {
      setIssuedCredential(output);
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const deleteMutation = useMutation({
    mutationFn: () => api.deleteAgent(id!),
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

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>节点详情</h1>
          <p className={styles.sectionCopy}>
            <AgentLabel id={agent.id} hostname={agent.hostname} ip={agent.ip} />
          </p>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton label="返回" onClick={() => navigate("/agents")} />
          <ActionButton label="刷新" onClick={() => agentQuery.refetch()} />
          <ActionButton label="启用" variant="ghost" pending={enableMutation.isPending} onClick={() => enableMutation.mutate()} />
          <ActionButton label="停用" variant="danger" pending={disableMutation.isPending} onClick={() => disableMutation.mutate()} />
          <ActionButton
            label="重置令牌"
            pending={resetMutation.isPending}
            pendingLabel="重置中"
            variant="primary"
            onClick={() => resetMutation.mutate()}
          />
          <ActionButton
            label="删除节点"
            pending={deleteMutation.isPending}
            pendingLabel="删除中"
            variant="danger"
            disabled={Boolean(deleteBlockedReason)}
            onClick={() => {
              if (window.confirm("确认删除这个节点？")) {
                deleteMutation.mutate();
              }
            }}
          />
        </div>
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

      <section className={styles.sectionCard}>
        <div className={styles.keyValueGrid}>
          <div className={styles.keyValue}>
            <span className={styles.key}>节点 ID</span>
            <span className={styles.value}>{agent.id}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>在线状态</span>
            <span className={styles.value}>{boolLabel(agent.online, "在线", "离线")}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>启用状态</span>
            <span className={styles.value}>{boolLabel(agent.enabled, "启用", "停用")}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>主机名</span>
            <span className={styles.value}>{agent.hostname || "—"}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>IP</span>
            <span className={styles.value}>{agent.ip || "—"}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>版本</span>
            <span className={styles.value}>{agent.version || "—"}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>最近连接</span>
            <span className={styles.value}>{formatDateTime(agent.lastConnectedAt)}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>最近心跳</span>
            <span className={styles.value}>{formatDateTime(agent.lastHeartbeatAt)}</span>
          </div>
        </div>
        <StatusPill label={agent.lastError || "最近没有错误"} tone={agent.lastError ? "danger" : "success"} />
      </section>
    </div>
  );
}
