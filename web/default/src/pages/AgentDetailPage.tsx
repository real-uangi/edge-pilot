import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { api, getErrorMessage, type AgentCredentialRecord } from "../lib/api";
import { formatDateTime, boolLabel } from "../lib/format";
import { ActionButton } from "../components/ActionButton";
import { AgentLabel } from "../components/AgentLabel";
import { useDialog } from "../components/DialogProvider";
import { StatusPill } from "../components/StatusPill";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../components/StateBlocks";
import styles from "../styles/admin.module.css";

export function AgentDetailPage() {
  const dialog = useDialog();
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
  const haproxyConfigQuery = useQuery({
    queryKey: ["agent-haproxy-config", id],
    queryFn: () => api.getAgentHAProxyConfig(id!),
    enabled: false,
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
            onClick={async () => {
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

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>HAProxy 实时配置</h2>
            <p className={styles.sectionCopy}>读取当前节点生效中的 haproxy.cfg。</p>
          </div>
          <div className={styles.buttonRow}>
            <ActionButton
              label="刷新配置"
              pending={haproxyConfigQuery.isFetching}
              pendingLabel="刷新中"
              onClick={() => haproxyConfigQuery.refetch()}
              disabled={agent.online !== true}
            />
          </div>
        </div>
        {agent.online !== true ? <InlineNotice message="节点离线，当前不可获取实时配置。" tone="info" /> : null}
        {haproxyConfigQuery.isError ? (
          <ErrorState
            title="配置读取失败"
            message={getErrorMessage(haproxyConfigQuery.error)}
            onRetry={() => haproxyConfigQuery.refetch()}
          />
        ) : null}
        {haproxyConfigQuery.isFetching ? <LoadingState title="正在读取配置" message="正在从在线节点拉取实时配置。" /> : null}
        {!haproxyConfigQuery.isFetching && haproxyConfigQuery.isSuccess && !haproxyConfigQuery.data.config.trim() ? (
          <EmptyState title="配置为空" message="节点返回了空配置内容。" />
        ) : null}
        {!haproxyConfigQuery.isFetching && haproxyConfigQuery.isSuccess && haproxyConfigQuery.data.config.trim() ? (
          <div className={styles.logCard}>
            <div className={styles.logMeta}>
              <span>刷新时间：{formatDateTime(haproxyConfigQuery.data.fetchedAt)}</span>
            </div>
            <pre className={styles.configBlock}>{haproxyConfigQuery.data.config}</pre>
          </div>
        ) : null}
      </section>
    </div>
  );
}
