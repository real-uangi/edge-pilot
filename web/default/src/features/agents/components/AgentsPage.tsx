import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime, boolLabel, shortId } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { CredentialResult } from "../../../shared/components/CredentialResult";
import { StatusPill } from "../../../shared/components/StatusPill";
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { agentsApi } from "../api";
import type { AgentCredentialRecord } from "../types";
import styles from "../../../styles/admin.module.css";

export function AgentsPage() {
  const queryClient = useQueryClient();
  const [credential, setCredential] = useState<AgentCredentialRecord | null>(
    null,
  );
  const [actionError, setActionError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: agentsApi.list,
    refetchInterval: 10000,
  });
  const createMutation = useMutation({
    mutationFn: async () => {
      const output = await agentsApi.create();
      setCredential(output);
    },
    onSuccess: async () => {
      setActionError(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["agents"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const query = search.trim().toLowerCase();
  const agents = (agentsQuery.data ?? []).filter(
    (agent) =>
      (!query ||
        [agent.id, agent.hostname, agent.ip, agent.version].some((value) =>
          value.toLowerCase().includes(query),
        )) &&
      (status === "all" ||
        (status === "online"
          ? agent.online === true
          : status === "offline"
            ? agent.online === false
            : agent.enabled === false)),
  );

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div className={styles.pageTitleGroup}>
          <h1 className={styles.sectionTitle}>节点</h1>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新"
            pending={agentsQuery.isFetching}
            onClick={() => void agentsQuery.refetch()}
          />
          <ActionButton
            label="新增节点"
            pending={createMutation.isPending}
            pendingLabel="创建中"
            disabled={Boolean(credential)}
            variant="primary"
            onClick={() => createMutation.mutate()}
          />
        </div>
      </header>
      {actionError ? <InlineNotice message={actionError} tone="error" /> : null}
      {credential ? (
        <CredentialResult
          id={credential.id}
          token={credential.token}
          onClose={() => setCredential(null)}
        />
      ) : null}
      <section className={styles.sectionCard}>
        <div className={styles.listToolbar}>
          <label className={styles.searchField}>
            <span className={styles.visuallyHidden}>搜索节点</span>
            <input
              className={styles.input}
              type="search"
              placeholder="搜索主机名、IP 或节点 ID"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <label className={styles.filterGroup}>
            <span className={styles.visuallyHidden}>节点状态</span>
            <select
              className={styles.select}
              value={status}
              onChange={(event) => setStatus(event.target.value)}
            >
              <option value="all">全部状态</option>
              <option value="online">在线</option>
              <option value="offline">离线</option>
              <option value="disabled">已停用</option>
            </select>
          </label>
          <span className={styles.resultCount} aria-live="polite">
            {agents.length} 个节点
          </span>
        </div>
        <QueryRefreshNotice
          query={agentsQuery}
          onRetry={() => void agentsQuery.refetch()}
        />
        {agentsQuery.isPending ? (
          <LoadingState title="正在加载节点" />
        ) : agentsQuery.isError && !agentsQuery.data ? (
          <ErrorState
            title="节点加载失败"
            message={getErrorMessage(agentsQuery.error)}
            onRetry={() => void agentsQuery.refetch()}
          />
        ) : !agentsQuery.data?.length ? (
          <EmptyState title="暂无节点" />
        ) : !agents.length ? (
          <EmptyState
            title="没有匹配的节点"
            actionLabel="清除筛选"
            onRetry={() => {
              setSearch("");
              setStatus("all");
            }}
          />
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.responsiveTable}>
              <thead>
                <tr>
                  <th>节点</th>
                  <th>IP</th>
                  <th>版本</th>
                  <th>连接</th>
                  <th>状态</th>
                  <th>最近心跳</th>
                </tr>
              </thead>
              <tbody>
                {agents.map((agent) => (
                  <tr key={agent.id}>
                    <td data-label="节点">
                      <Link
                        className={styles.tableLink}
                        to={`/agents/${agent.id}`}
                      >
                        {agent.hostname || shortId(agent.id)}
                      </Link>
                      <div className={styles.muted}>
                        <code>{shortId(agent.id)}</code>
                      </div>
                    </td>
                    <td data-label="IP">
                      <code>{agent.ip || "—"}</code>
                    </td>
                    <td data-label="版本">
                      <code>{agent.version || "—"}</code>
                    </td>
                    <td data-label="连接">
                      <StatusPill
                        label={boolLabel(agent.online, "在线", "离线")}
                        tone={
                          agent.online === true
                            ? "success"
                            : agent.online === false
                              ? "danger"
                              : "default"
                        }
                      />
                    </td>
                    <td data-label="状态">
                      <StatusPill
                        label={boolLabel(agent.enabled, "启用", "停用")}
                        tone={agent.enabled === false ? "warning" : "default"}
                      />
                    </td>
                    <td data-label="最近心跳">
                      {formatDateTime(agent.lastHeartbeatAt)}
                    </td>
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
