import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import {
  formatDateTime,
  releaseStatusLabel,
  releaseStatusTone,
} from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { StatusPill } from "../../../shared/components/StatusPill";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { releasesApi } from "../api";
import { agentsApi } from "../../agents/api";
import styles from "../../../styles/admin.module.css";

export function ReleasesPage() {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const releasesQuery = useQuery({
    queryKey: ["releases"],
    queryFn: releasesApi.list,
  });
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: agentsApi.list,
  });
  const agentsByID = new Map(
    (agentsQuery.data ?? []).map((agent) => [agent.id, agent] as const),
  );
  const term = search.trim().toLowerCase();
  const filteredReleases = (releasesQuery.data ?? []).filter(
    (release) =>
      (!status || release.status === Number(status)) &&
      (!term ||
        [
          release.id,
          release.imageRepo,
          release.imageTag,
          release.triggeredBy,
        ].some((value) => value.toLowerCase().includes(term))),
  );
  const clearFilters = () => {
    setSearch("");
    setStatus("");
  };

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div className={styles.pageTitleGroup}>
          <h1 className={styles.sectionTitle}>发布</h1>
          <span className={styles.resultCount}>
            {releasesQuery.data?.length ?? 0} 个发布单
          </span>
        </div>
        <ActionButton
          label="刷新"
          pending={releasesQuery.isFetching}
          onClick={() => void releasesQuery.refetch()}
          variant="ghost"
        />
      </header>
      <QueryRefreshNotice
        query={releasesQuery}
        onRetry={() => void releasesQuery.refetch()}
      />
      <section className={styles.sectionCard} aria-label="发布列表">
        <div className={styles.filterBar}>
          <label className={styles.filterField}>
            <span className={styles.visuallyHidden}>搜索发布单</span>
            <input
              className={styles.input}
              type="search"
              placeholder="搜索发布单、镜像或触发人"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <label className={styles.filterField}>
            <span className={styles.visuallyHidden}>发布状态</span>
            <select
              className={styles.select}
              value={status}
              onChange={(event) => setStatus(event.target.value)}
            >
              <option value="">全部状态</option>
              {[1, 2, 3, 4, 5, 6, 7, 8, 9].map((value) => (
                <option key={value} value={value}>
                  {releaseStatusLabel(value)}
                </option>
              ))}
            </select>
          </label>
          <span className={styles.resultCount} role="status">
            {filteredReleases.length} / {releasesQuery.data?.length ?? 0}
          </span>
        </div>
        {releasesQuery.isPending ? (
          <LoadingState title="正在加载发布单" />
        ) : releasesQuery.isError && !releasesQuery.data ? (
          <ErrorState
            title="发布单加载失败"
            message={getErrorMessage(releasesQuery.error)}
            onRetry={() => releasesQuery.refetch()}
          />
        ) : !releasesQuery.data?.length ? (
          <EmptyState
            title="暂无发布单"
            message="从服务详情选择镜像 Tag 创建发布单。"
          />
        ) : !filteredReleases.length ? (
          <div className={styles.emptyActions}>
            <EmptyState title="没有匹配的发布单" />
            <ActionButton
              label="清除筛选"
              onClick={clearFilters}
              variant="ghost"
            />
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.responsiveTable}>
              <thead>
                <tr>
                  <th>发布单</th>
                  <th>状态</th>
                  <th>镜像</th>
                  <th>节点</th>
                  <th>队列位置</th>
                  <th>创建时间</th>
                </tr>
              </thead>
              <tbody>
                {filteredReleases.map((release) => (
                  <tr key={release.id}>
                    <td data-label="发布单">
                      <Link
                        className={`${styles.cellPrimary} ${styles.mono}`}
                        to={`/releases/${release.id}`}
                      >
                        {release.id.slice(0, 8)}
                      </Link>
                      <span className={styles.cellSecondary}>
                        {release.triggeredBy || "—"}
                      </span>
                    </td>
                    <td data-label="状态">
                      <StatusPill
                        label={releaseStatusLabel(release.status)}
                        tone={releaseStatusTone(
                          release.status,
                          release.isActive,
                        )}
                      />
                    </td>
                    <td data-label="镜像">
                      <span className={`${styles.cellPrimary} ${styles.mono}`}>
                        {release.imageTag}
                      </span>
                      <span
                        className={`${styles.cellSecondary} ${styles.mono}`}
                      >
                        {release.imageRepo}
                      </span>
                    </td>
                    <td data-label="节点">
                      <AgentLabel
                        id={release.agentId}
                        hostname={agentsByID.get(release.agentId)?.hostname}
                        ip={agentsByID.get(release.agentId)?.ip}
                      />
                    </td>
                    <td data-label="队列位置">
                      {release.queuePosition > 0 ? release.queuePosition : "—"}
                    </td>
                    <td data-label="创建时间">
                      {formatDateTime(release.createdAt)}
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
