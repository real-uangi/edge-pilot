import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, getErrorMessage } from "../lib/api";
import { boolLabel, formatDateTime, releaseStatusLabel, releaseStatusTone } from "../lib/format";
import { AgentLabel } from "../components/AgentLabel";
import { StatusPill } from "../components/StatusPill";
import { ErrorState, LoadingState } from "../components/StateBlocks";
import styles from "../styles/admin.module.css";

export function DashboardPage() {
  const overviewQuery = useQuery({
    queryKey: ["overview"],
    queryFn: api.overview,
    refetchInterval: 10000,
  });

  if (overviewQuery.isPending) {
    return (
      <div className={styles.page}>
        <LoadingState title="正在加载总览" message="正在汇总服务、节点和发布信息。" />
      </div>
    );
  }

  if (overviewQuery.isError) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="总览加载失败"
          message={getErrorMessage(overviewQuery.error)}
          onRetry={() => overviewQuery.refetch()}
        />
      </div>
    );
  }

  if (!overviewQuery.data) {
    return (
      <div className={styles.page}>
        <ErrorState title="总览暂不可用" message="当前未返回总览数据，请稍后重试。" />
      </div>
    );
  }

  const { agents, services, recentReleases, activeInstances } = overviewQuery.data;
  const agentsByID = new Map(agents.map((agent) => [agent.id, agent] as const));
  const onlineAgents = agents.filter((item) => item.online).length;
  const enabledServices = services.filter((item) => item.enabled).length;
  const activeReleases = recentReleases.filter((item) => item.isActive).length;

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>总览</h1>
          <p className={styles.sectionCopy}>集中查看服务、节点与发布状态。</p>
        </div>
      </section>

      <section className={styles.cardGrid}>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>运行实例</span>
          <span className={styles.metricValue}>{activeInstances}</span>
          <span className={styles.metricMeta}>当前运行中的受管实例</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>在线节点</span>
          <span className={styles.metricValue}>{onlineAgents}</span>
          <span className={styles.metricMeta}>总数 {agents.length}</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>启用服务</span>
          <span className={styles.metricValue}>{enabledServices}</span>
          <span className={styles.metricMeta}>总数 {services.length}</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>活动发布</span>
          <span className={styles.metricValue}>{activeReleases}</span>
          <span className={styles.metricMeta}>当前活动中的发布单</span>
        </article>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>服务目录</h2>
          </div>
          <Link className={styles.primaryButton} to="/services">
            查看服务
          </Link>
        </div>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>名称</th>
                <th>路由</th>
                <th>节点</th>
                <th>启用</th>
                <th>更新时间</th>
              </tr>
            </thead>
            <tbody>
              {services.map((service) => (
                <tr key={service.id}>
                  <td>
                    <Link className={styles.tableLink} to={`/services/${service.id}`}>
                      {service.name}
                    </Link>
                  </td>
                  <td>{service.routeHost + service.routePathPrefix}</td>
                  <td>
                    <AgentLabel
                      id={service.agentId}
                      hostname={agentsByID.get(service.agentId)?.hostname}
                      ip={agentsByID.get(service.agentId)?.ip}
                    />
                  </td>
                  <td>{boolLabel(service.enabled, "启用", "停用")}</td>
                  <td>{formatDateTime(service.updatedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className={styles.split}>
        <div className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className={styles.sectionTitle}>发布流</h2>
            </div>
            <Link className={styles.secondaryButton} to="/releases">
              查看发布
            </Link>
          </div>
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>发布单</th>
                  <th>状态</th>
                  <th>镜像</th>
                  <th>创建时间</th>
                </tr>
              </thead>
              <tbody>
                {recentReleases.map((release) => (
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
                    <td>{formatDateTime(release.createdAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        <div className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className={styles.sectionTitle}>节点态势</h2>
            </div>
            <Link className={styles.secondaryButton} to="/agents">
              查看节点
            </Link>
          </div>
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>IP</th>
                  <th>在线</th>
                  <th>启用</th>
                  <th>最近心跳</th>
                </tr>
              </thead>
              <tbody>
                {agents.map((agent) => (
                  <tr key={agent.id}>
                    <td>
                      <Link className={styles.tableLink} to={`/agents/${agent.id}`}>
                        <AgentLabel id={agent.id} hostname={agent.hostname} ip={agent.ip} />
                      </Link>
                    </td>
                    <td>{agent.ip || "—"}</td>
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
        </div>
      </section>
    </div>
  );
}
