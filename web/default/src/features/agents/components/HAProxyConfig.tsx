import { useQuery } from "@tanstack/react-query";
import { formatDateTime } from "../../../shared/lib/format";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { ActionButton } from "../../../shared/components/ActionButton";
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { agentsApi } from "../api";
import styles from "../../../styles/admin.module.css";

interface HAProxyConfigProps {
  agentId: string;
  agentOnline: boolean | null;
}

export function HAProxyConfig({ agentId, agentOnline }: HAProxyConfigProps) {
  const haproxyConfigQuery = useQuery({
    queryKey: ["agent-haproxy-config", agentId],
    queryFn: () => agentsApi.getHAProxyConfig(agentId),
    enabled: false,
  });

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>HAProxy 实时配置</h2>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新配置"
            pending={haproxyConfigQuery.isFetching}
            pendingLabel="刷新中"
            onClick={() => haproxyConfigQuery.refetch()}
            disabled={agentOnline !== true}
          />
        </div>
      </div>
      {agentOnline !== true ? (
        <InlineNotice message="节点离线，当前不可获取实时配置。" tone="info" />
      ) : null}
      {haproxyConfigQuery.data ? (
        <QueryRefreshNotice
          query={haproxyConfigQuery}
          onRetry={() => void haproxyConfigQuery.refetch()}
        />
      ) : null}
      {haproxyConfigQuery.isError && !haproxyConfigQuery.data ? (
        <ErrorState
          title="配置读取失败"
          message={getErrorMessage(haproxyConfigQuery.error)}
          onRetry={
            agentOnline === true
              ? () => void haproxyConfigQuery.refetch()
              : undefined
          }
        />
      ) : null}
      {haproxyConfigQuery.isFetching && !haproxyConfigQuery.data ? (
        <LoadingState title="正在读取配置" />
      ) : null}
      {!haproxyConfigQuery.isFetching &&
      !haproxyConfigQuery.data &&
      !haproxyConfigQuery.isError ? (
        <EmptyState title="尚未读取实时配置" />
      ) : null}
      {haproxyConfigQuery.data && !haproxyConfigQuery.data.config.trim() ? (
        <EmptyState title="配置为空" message="节点返回了空配置内容。" />
      ) : null}
      {haproxyConfigQuery.data?.config.trim() ? (
        <div className={styles.logCard}>
          <div className={styles.logMeta}>
            <span>
              刷新时间：{formatDateTime(haproxyConfigQuery.data.fetchedAt)}
            </span>
          </div>
          <pre className={styles.configBlock}>
            {haproxyConfigQuery.data.config}
          </pre>
        </div>
      ) : null}
    </section>
  );
}
