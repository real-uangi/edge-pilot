import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { boolLabel, formatDateTime, releaseStatusLabel, releaseStatusTone } from "../lib/format";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function DashboardPage() {
  const overviewQuery = useQuery({
    queryKey: ["overview"],
    queryFn: api.overview,
    refetchInterval: 10000,
  });

  if (overviewQuery.isPending) {
    return <div className={styles.page}>正在加载总览…</div>;
  }
  if (!overviewQuery.data) {
    return <div className={styles.page}>总览暂不可用。</div>;
  }

  const { agents, services, recentReleases, activeInstances } = overviewQuery.data;
  const onlineAgents = agents.filter((item) => item.online).length;
  const enabledServices = services.filter((item) => item.enabled).length;
  const activeReleases = recentReleases.filter((item) => item.isActive).length;

  return (
    <div className={styles.page}>
      <section className={styles.hero}>
        <span className={styles.eyebrow}>总览</span>
        <h1 className={styles.title}>把单机服务运行成一套可控的发布面板</h1>
        <p className={styles.subtitle}>
          这里集中看服务配置、节点存活、发布流转和运行态实例，信息更紧凑，动作更直接。
        </p>
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
          <span className={styles.metricMeta}>注册总数 {agents.length}</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>启用服务</span>
          <span className={styles.metricValue}>{enabledServices}</span>
          <span className={styles.metricMeta}>服务总数 {services.length}</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>活动发布</span>
          <span className={styles.metricValue}>{activeReleases}</span>
          <span className={styles.metricMeta}>最近发布中的活动单</span>
        </article>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>服务</h2>
            <p className={styles.sectionCopy}>优先看路由、归属节点和启用状态。</p>
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
                  <td>{service.agentId}</td>
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
              <h2 className={styles.sectionTitle}>节点</h2>
              <p className={styles.sectionCopy}>轮询在线态和最近心跳。</p>
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
                        {agent.id}
                      </Link>
                    </td>
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

        <div className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className={styles.sectionTitle}>最近发布</h2>
              <p className={styles.sectionCopy}>排队、部署、切流和回滚都在这里看。</p>
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
      </section>
    </div>
  );
}
