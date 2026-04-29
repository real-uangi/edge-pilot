import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime, releaseStatusLabel, releaseStatusTone } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { releasesApi } from "../api";
import { agentsApi } from "../../agents/api";
import styles from "../../../styles/admin.module.css";

export function ReleasesPage() {
  const releasesQuery = useQuery({
    queryKey: ["releases"],
    queryFn: releasesApi.list,
  });
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: agentsApi.list,
  });
  const agentsByID = new Map((agentsQuery.data ?? []).map((agent) => [agent.id, agent] as const));

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>发布</h1>
          <p className={styles.sectionCopy}>跟踪发布单进度与节点执行状态。</p>
        </div>
        <ActionButton label="刷新" onClick={() => releasesQuery.refetch()} />
      </section>

      <section className={styles.sectionCard}>
        {releasesQuery.isPending ? (
          <LoadingState title="正在加载发布列表" message="正在拉取最近发布记录。" />
        ) : releasesQuery.isError ? (
          <ErrorState
            title="发布列表加载失败"
            message={getErrorMessage(releasesQuery.error)}
            onRetry={() => releasesQuery.refetch()}
          />
        ) : !releasesQuery.data?.length ? (
          <EmptyState title="暂无发布单" message="当前还没有发布记录。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>发布单</th>
                  <th>状态</th>
                  <th>镜像</th>
                  <th>节点</th>
                  <th>队列</th>
                  <th>创建时间</th>
                </tr>
              </thead>
              <tbody>
                {releasesQuery.data.map((release) => (
                  <tr key={release.id}>
                    <td>
                      <Link className={styles.tableLink} to={`/releases/${release.id}`}>
                        {release.id.slice(0, 8)}
                      </Link>
                    </td>
                    <td>
                      <StatusPill
                        label={releaseStatusLabel(release.status)}
                        tone={releaseStatusTone(release.status, release.isActive)}
                      />
                    </td>
                    <td>{release.imageRepo + ":" + release.imageTag}</td>
                    <td>
                      <AgentLabel
                        id={release.agentId}
                        hostname={agentsByID.get(release.agentId)?.hostname}
                        ip={agentsByID.get(release.agentId)?.ip}
                      />
                    </td>
                    <td>{release.queuePosition}</td>
                    <td>{formatDateTime(release.createdAt)}</td>
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
