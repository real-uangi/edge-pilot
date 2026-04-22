import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { UnifiedLineChart, type ChartSeries } from "../components/UnifiedLineChart";
import { EmptyState, ErrorState, LoadingState } from "../components/StateBlocks";
import { api, getErrorMessage, type PerformancePoint } from "../lib/api";
import { boolLabel, formatAgentLabel, formatBytes, formatDateTime, formatPercent } from "../lib/format";
import styles from "../styles/admin.module.css";

export function SystemPerformancePage() {
  const [selectedAgentID, setSelectedAgentID] = useState<string | null>(null);
  const overviewQuery = useQuery({
    queryKey: ["system-performance"],
    queryFn: api.getSystemPerformanceOverview,
    refetchInterval: 10000,
  });

  useEffect(() => {
    if (!overviewQuery.data?.agents.length) {
      setSelectedAgentID(null);
      return;
    }
    if (!selectedAgentID || !overviewQuery.data.agents.some((item) => item.id === selectedAgentID)) {
      setSelectedAgentID(overviewQuery.data.agents[0].id);
    }
  }, [overviewQuery.data, selectedAgentID]);

  const agentHistoryQuery = useQuery({
    queryKey: ["agent-performance-history", selectedAgentID],
    queryFn: () => api.getAgentPerformanceHistory(selectedAgentID!),
    enabled: Boolean(selectedAgentID),
    refetchInterval: 10000,
  });

  if (overviewQuery.isPending) {
    return (
      <div className={styles.page}>
        <LoadingState title="正在加载系统性能" message="正在同步 ControlPlane 与 Agent 指标快照。" />
      </div>
    );
  }

  if (overviewQuery.isError) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="系统性能加载失败"
          message={getErrorMessage(overviewQuery.error)}
          onRetry={() => overviewQuery.refetch()}
        />
      </div>
    );
  }

  if (!overviewQuery.data) {
    return (
      <div className={styles.page}>
        <EmptyState title="系统性能暂不可用" message="当前未返回可展示的性能数据。" />
      </div>
    );
  }

  const controlPlaneLatest = overviewQuery.data.controlPlaneLatest;
  const controlPlaneHistory = overviewQuery.data.controlPlaneHistory;
  const selectedAgent = overviewQuery.data.agents.find((item) => item.id === selectedAgentID) ?? null;

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>系统性能</h1>
          <p className={styles.sectionCopy}>ControlPlane 与 Agent 的 CPU/内存趋势，保留最近 240 个采样点。</p>
        </div>
      </section>

      <section className={styles.cardGrid}>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>ControlPlane CPU</span>
          <span className={styles.metricValue}>{formatPercent(controlPlaneLatest?.cpuPercent)}</span>
          <span className={styles.metricMeta}>采集来源：{controlPlaneLatest?.source || "—"}</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>ControlPlane 内存</span>
          <span className={styles.metricValue}>{formatBytes(controlPlaneLatest?.memoryUsedBytes)}</span>
          <span className={styles.metricMeta}>
            限制：{controlPlaneLatest?.memoryLimitBytes ? formatBytes(controlPlaneLatest.memoryLimitBytes) : "无限制"}
          </span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>采样点</span>
          <span className={styles.metricValue}>{controlPlaneHistory.length}</span>
          <span className={styles.metricMeta}>环形缓存容量 240</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>最近采样</span>
          <span className={styles.metricValue}>{formatDateTime(controlPlaneLatest?.collectedAt)}</span>
          <span className={styles.metricMeta}>每 15 秒采样</span>
        </article>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>ControlPlane 趋势</h2>
            <p className={styles.sectionCopy}>CPU 与内存趋势图（最近 240 点）。</p>
          </div>
        </div>
        {controlPlaneHistory.length ? (
          <div className={styles.split}>
            <UnifiedLineChart
              title="CPU 使用率"
              series={toChartSeries(controlPlaneHistory, (item) => round(item.cpuPercent), "CPU")}
              yValueFormatter={formatPercentValue}
              tooltipValueFormatter={formatPercentValue}
            />
            <UnifiedLineChart
              title="内存使用"
              series={toChartSeries(controlPlaneHistory, (item) => round(item.memoryUsedBytes / 1024 / 1024), "Memory")}
              yValueFormatter={formatMibValue}
              tooltipValueFormatter={formatMibValue}
            />
          </div>
        ) : (
          <EmptyState title="暂无 ControlPlane 历史数据" message="等待采样后自动展示趋势。" />
        )}
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>Agent 性能快照</h2>
            <p className={styles.sectionCopy}>点击行可查看该 Agent 历史趋势（按需加载）。</p>
          </div>
        </div>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>节点</th>
                <th>在线</th>
                <th>CPU</th>
                <th>内存</th>
                <th>来源</th>
                <th>采样时间</th>
              </tr>
            </thead>
            <tbody>
              {overviewQuery.data.agents.map((agent) => (
                <tr
                  key={agent.id}
                  className={selectedAgentID === agent.id ? styles.tableRowSelected : styles.tableRowInteractive}
                  onClick={() => setSelectedAgentID(agent.id)}
                >
                  <td>{formatAgentLabel({ id: agent.id, hostname: agent.hostname, ip: agent.ip })}</td>
                  <td>{boolLabel(agent.online, "在线", "离线")}</td>
                  <td>{formatPercent(agent.latest?.cpuPercent)}</td>
                  <td>
                    {agent.latest
                      ? `${formatBytes(agent.latest.memoryUsedBytes)} / ${
                          agent.latest.memoryLimitBytes ? formatBytes(agent.latest.memoryLimitBytes) : "无限制"
                        }`
                      : "—"}
                  </td>
                  <td>{agent.latest?.source || "—"}</td>
                  <td>{formatDateTime(agent.latest?.collectedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>Agent 历史趋势</h2>
            <p className={styles.sectionCopy}>
              {selectedAgent
                ? formatAgentLabel({ id: selectedAgent.id, hostname: selectedAgent.hostname, ip: selectedAgent.ip })
                : "请选择一个 Agent"}
            </p>
          </div>
        </div>
        {!selectedAgentID ? (
          <EmptyState title="暂无可选 Agent" message="请先创建并连接 Agent 节点。" />
        ) : agentHistoryQuery.isPending ? (
          <LoadingState title="正在加载 Agent 历史" message="正在获取该节点的环形快照数据。" />
        ) : agentHistoryQuery.isError ? (
          <ErrorState
            title="Agent 历史加载失败"
            message={getErrorMessage(agentHistoryQuery.error)}
            onRetry={() => agentHistoryQuery.refetch()}
          />
        ) : !agentHistoryQuery.data?.history.length ? (
          <EmptyState title="暂无 Agent 历史数据" message="该节点尚未上报自身性能快照。" />
        ) : (
          <div className={styles.split}>
            <UnifiedLineChart
              title="CPU 使用率"
              series={toChartSeries(agentHistoryQuery.data.history, (item) => round(item.cpuPercent), "CPU")}
              yValueFormatter={formatPercentValue}
              tooltipValueFormatter={formatPercentValue}
            />
            <UnifiedLineChart
              title="内存使用"
              series={toChartSeries(
                agentHistoryQuery.data.history,
                (item) => round(item.memoryUsedBytes / 1024 / 1024),
                "Memory",
              )}
              yValueFormatter={formatMibValue}
              tooltipValueFormatter={formatMibValue}
            />
          </div>
        )}
      </section>
    </div>
  );
}

function toChartSeries(
  history: PerformancePoint[],
  selector: (value: PerformancePoint) => number,
  id: string,
): ChartSeries[] {
  return [
    {
      id,
      data: history
        .filter((point) => Boolean(point.collectedAt))
        .map((point) => ({
          x: point.collectedAt,
          y: selector(point),
        })),
    },
  ];
}

function round(value: number): number {
  return Math.round(value * 100) / 100;
}

function formatPercentValue(value: number): string {
  return `${round(value)}%`;
}

function formatMibValue(value: number): string {
  return `${round(value)} MiB`;
}
