import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, getErrorMessage, type AgentCredentialRecord } from "../lib/api";
import { formatDateTime, boolLabel } from "../lib/format";
import { ActionButton } from "../components/ActionButton";
import { AgentLabel } from "../components/AgentLabel";
import { StatusPill } from "../components/StatusPill";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../components/StateBlocks";
import styles from "../styles/admin.module.css";

export function AgentsPage() {
  const queryClient = useQueryClient();
  const [credential, setCredential] = useState<AgentCredentialRecord | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: api.listAgents,
    refetchInterval: 10000,
  });

  const createMutation = useMutation({
    mutationFn: api.createAgent,
    onSuccess: async (output) => {
      setCredential(output);
      setActionError(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["agents"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>节点</h1>
          <p className={styles.sectionCopy}>管理节点凭据、在线状态与可用性。</p>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton label="刷新" onClick={() => agentsQuery.refetch()} />
          <ActionButton
            label="新增节点"
            pending={createMutation.isPending}
            pendingLabel="创建中"
            variant="primary"
            onClick={() => createMutation.mutate()}
          />
        </div>
      </section>

      {actionError ? <InlineNotice message={actionError} tone="error" /> : null}

      {credential ? (
        <section className={styles.credentialCard}>
          <span className={styles.eyebrow}>新签发凭据</span>
          <div>
            <strong>ID</strong>
            <div className={styles.credentialValue}>{credential.id}</div>
          </div>
          <div>
            <strong>令牌</strong>
            <div className={styles.credentialValue}>{credential.token}</div>
          </div>
        </section>
      ) : null}

      <section className={styles.sectionCard}>
        {agentsQuery.isPending ? (
          <LoadingState title="正在加载节点列表" message="正在同步节点在线状态。" />
        ) : agentsQuery.isError ? (
          <ErrorState
            title="节点列表加载失败"
            message={getErrorMessage(agentsQuery.error)}
            onRetry={() => agentsQuery.refetch()}
          />
        ) : !agentsQuery.data?.length ? (
          <EmptyState title="暂无节点" message="当前没有可管理节点，可先新增节点并下发凭据。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>节点</th>
                  <th>主机名</th>
                  <th>IP</th>
                  <th>版本</th>
                  <th>在线</th>
                  <th>启用</th>
                  <th>最近心跳</th>
                </tr>
              </thead>
              <tbody>
                {agentsQuery.data.map((agent) => (
                  <tr key={agent.id}>
                    <td>
                      <Link className={styles.tableLink} to={`/agents/${agent.id}`}>
                        <AgentLabel id={agent.id} hostname={agent.hostname} ip={agent.ip} />
                      </Link>
                    </td>
                    <td>{agent.hostname || "—"}</td>
                    <td>{agent.ip || "—"}</td>
                    <td>{agent.version || "—"}</td>
                    <td>
                      <StatusPill
                        label={boolLabel(agent.online, "在线", "离线")}
                        tone={agent.online ? "success" : "danger"}
                      />
                    </td>
                    <td>{boolLabel(agent.enabled, "启用", "停用")}</td>
                    <td>{formatDateTime(agent.lastHeartbeatAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
