import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  UnifiedLineChart,
  type ChartPoint,
} from "../../../shared/components/UnifiedLineChart";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { ActionButton } from "../../../shared/components/ActionButton";
import { StatusPill } from "../../../shared/components/StatusPill";
import { getErrorMessage } from "../../../shared/lib/api-client";
import {
  boolLabel,
  formatBytes,
  formatDateTime,
  formatPercent,
} from "../../../shared/lib/format";
import { systemPerformanceApi } from "../api";
import type { PerformancePoint } from "../types";
import styles from "../../../styles/admin.module.css";
import performance from "./SystemPerformancePage.module.css";

export function SystemPerformancePage() {
  const [selectedAgentID, setSelectedAgentID] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const overviewQuery = useQuery({
    queryKey: ["system-performance"],
    queryFn: systemPerformanceApi.getOverview,
    refetchInterval: 10000,
  });
  useEffect(() => {
    const agents = overviewQuery.data?.agents;
    if (!agents) return;
    if (!agents.some((agent) => agent.id === selectedAgentID))
      setSelectedAgentID(agents[0]?.id ?? null);
  }, [overviewQuery.data, selectedAgentID]);
  const historyQuery = useQuery({
    queryKey: ["agent-performance-history", selectedAgentID],
    queryFn: () => systemPerformanceApi.getAgentHistory(selectedAgentID!),
    enabled: Boolean(selectedAgentID),
    refetchInterval: 10000,
  });
  if (overviewQuery.isPending) return <LoadingState title="正在加载系统性能" />;
  if (!overviewQuery.data)
    return (
      <ErrorState
        title="性能数据加载失败"
        message={getErrorMessage(overviewQuery.error)}
        onRetry={() => void overviewQuery.refetch()}
      />
    );
  const {
    controlPlaneLatest: latest,
    controlPlaneHistory: history,
    agents,
  } = overviewQuery.data;
  const selected = agents.find((agent) => agent.id === selectedAgentID);
  const filtered = agents.filter((agent) =>
    [agent.hostname, agent.ip, agent.id].some((value) =>
      value.toLowerCase().includes(search.trim().toLowerCase()),
    ),
  );
  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <h1 className={styles.sectionTitle}>系统性能</h1>
        <ActionButton
          label="刷新"
          pending={overviewQuery.isFetching}
          onClick={() => void overviewQuery.refetch()}
        />
      </header>
      <section className={styles.cardGrid}>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>Control Plane CPU</span>
          <span className={styles.metricValue}>
            {formatPercent(latest?.cpuPercent)}
          </span>
          <span className={styles.metricMeta}>
            来源 {latest?.source || "—"}
          </span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>Control Plane 内存</span>
          <span className={styles.metricValue}>
            {formatBytes(latest?.memoryUsedBytes)}
          </span>
          <span className={styles.metricMeta}>
            限制{" "}
            {latest?.memoryLimitBytes
              ? formatBytes(latest.memoryLimitBytes)
              : "—"}
          </span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>历史采样</span>
          <span className={styles.metricValue}>{history.length}</span>
          <span className={styles.metricMeta}>
            最近采样 {formatDateTime(latest?.collectedAt)}
          </span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>上报节点</span>
          <span className={styles.metricValue}>
            {agents.filter((agent) => agent.latest).length}
          </span>
          <span className={styles.metricMeta}>共 {agents.length} 个节点</span>
        </article>
      </section>
      <QueryRefreshNotice
        query={overviewQuery}
        onRetry={() => void overviewQuery.refetch()}
      />
      <section className={styles.sectionCard}>
        <h2 className={styles.sectionTitle}>Control Plane 趋势</h2>
        <PerformanceCharts history={history} />
      </section>
      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>节点性能</h2>
          <span className={styles.resultCount}>
            {filtered.length} / {agents.length} 个节点
          </span>
        </div>
        <label className={styles.field}>
          <span className={styles.visuallyHidden}>搜索节点</span>
          <input
            type="search"
            className={styles.input}
            placeholder="搜索节点名称、IP 或 ID"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </label>
        {!filtered.length ? (
          <EmptyState
            title={agents.length ? "没有匹配的节点" : "暂无节点"}
            actionLabel={search ? "清除搜索" : undefined}
            onRetry={search ? () => setSearch("") : undefined}
          />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>节点</th>
                  <th>状态</th>
                  <th>CPU</th>
                  <th>内存 / 限制</th>
                  <th>采集来源</th>
                  <th>采样时间</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((agent) => (
                  <tr
                    key={agent.id}
                    className={
                      selectedAgentID === agent.id
                        ? styles.tableRowSelected
                        : ""
                    }
                  >
                    <td>
                      <button
                        className={styles.tableLink}
                        type="button"
                        aria-pressed={selectedAgentID === agent.id}
                        aria-controls="agent-performance-history"
                        onClick={() => setSelectedAgentID(agent.id)}
                      >
                        {agent.hostname || agent.id.slice(0, 8)}
                      </button>
                      <div className={styles.hint}>{agent.ip || "—"}</div>
                    </td>
                    <td>
                      <StatusPill
                        label={boolLabel(agent.online, "在线", "离线")}
                        tone={
                          agent.online == null
                            ? "default"
                            : agent.online
                              ? "success"
                              : "danger"
                        }
                      />
                    </td>
                    <td>{formatPercent(agent.latest?.cpuPercent)}</td>
                    <td>
                      {formatBytes(agent.latest?.memoryUsedBytes)} /{" "}
                      {agent.latest
                        ? agent.latest.memoryLimitBytes
                          ? formatBytes(agent.latest.memoryLimitBytes)
                          : "无限制"
                        : "—"}
                    </td>
                    <td>{agent.latest?.source || "—"}</td>
                    <td>{formatDateTime(agent.latest?.collectedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <section
        className={styles.sectionCard}
        id="agent-performance-history"
        aria-label="所选节点历史趋势"
      >
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>节点历史趋势</h2>
            <p className={styles.sectionCopy}>
              {selected?.hostname ||
                selected?.ip ||
                selectedAgentID ||
                "选择节点查看趋势"}
            </p>
          </div>
        </div>
        {!selectedAgentID ? (
          <EmptyState title="暂无可选节点" />
        ) : historyQuery.isPending ? (
          <LoadingState title="正在加载节点历史" />
        ) : !historyQuery.data ? (
          <ErrorState
            title="节点历史加载失败"
            message={getErrorMessage(historyQuery.error)}
            onRetry={() => void historyQuery.refetch()}
          />
        ) : (
          <>
            <QueryRefreshNotice
              query={historyQuery}
              onRetry={() => void historyQuery.refetch()}
            />
            <PerformanceCharts history={historyQuery.data.history} />
          </>
        )}
      </section>
    </div>
  );
}

function PerformanceCharts({ history }: { history: PerformancePoint[] }) {
  if (!history.length)
    return (
      <EmptyState title="暂无趋势数据" message="等待节点采样后自动更新。" />
    );
  const cpu: ChartPoint[] = history.map((point) => ({
    x: point.collectedAt,
    y: Math.round(point.cpuPercent * 100) / 100,
  }));
  const memory: ChartPoint[] = history.map((point) => ({
    x: point.collectedAt,
    y: Math.round((point.memoryUsedBytes / 1024 / 1024) * 100) / 100,
  }));
  return (
    <div className={performance.chartGrid}>
      <UnifiedLineChart
        title="CPU"
        id="CPU"
        data={cpu}
        yUnit="%"
        tooltipValueFormatter={(value) => `${value.toFixed(1)}%`}
      />
      <UnifiedLineChart
        title="内存"
        id="内存"
        data={memory}
        yUnit=" MiB"
        tooltipValueFormatter={(value) => `${value.toFixed(1)} MiB`}
        color="#3b7861"
      />
    </div>
  );
}
