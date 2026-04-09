import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { boolLabel, formatDateTime, releaseStatusLabel } from "../lib/format";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function DashboardPage() {
  const overviewQuery = useQuery({
    queryKey: ["overview"],
    queryFn: api.overview,
    refetchInterval: 10000,
  });

  if (overviewQuery.isPending) {
    return <div className={styles.page}>Loading overview…</div>;
  }
  if (!overviewQuery.data) {
    return <div className={styles.page}>Overview unavailable.</div>;
  }

  const { agents, services, recentReleases, activeInstances } = overviewQuery.data;
  const onlineAgents = agents.filter((item) => item.online).length;
  const enabledServices = services.filter((item) => item.enabled).length;
  const activeReleases = recentReleases.filter((item) => item.isActive).length;

  return (
    <div className={styles.page}>
      <section className={styles.hero}>
        <span className={styles.eyebrow}>Overview</span>
        <h1 className={styles.title}>Operate One Box Like A Real Control Plane</h1>
        <p className={styles.subtitle}>
          这里集中看服务编排、Agent 存活、发布流转和运行态实例，不把平台策略写回页面，只给动作和状态。
        </p>
      </section>

      <section className={styles.cardGrid}>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>Active Instances</span>
          <span className={styles.metricValue}>{activeInstances}</span>
          <span className={styles.metricMeta}>当前运行中的受管实例</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>Online Agents</span>
          <span className={styles.metricValue}>{onlineAgents}</span>
          <span className={styles.metricMeta}>{agents.length} agents in registry</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>Enabled Services</span>
          <span className={styles.metricValue}>{enabledServices}</span>
          <span className={styles.metricMeta}>{services.length} services tracked</span>
        </article>
        <article className={styles.statCard}>
          <span className={styles.metricLabel}>Active Releases</span>
          <span className={styles.metricValue}>{activeReleases}</span>
          <span className={styles.metricMeta}>最近发布中的活动单</span>
        </article>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>Services</h2>
            <p className={styles.sectionCopy}>优先看路由、归属 Agent 和启用状态。</p>
          </div>
          <Link className={styles.primaryButton} to="/services">
            Open Services
          </Link>
        </div>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Route</th>
                <th>Agent</th>
                <th>Enabled</th>
                <th>Updated</th>
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
                  <td>{boolLabel(service.enabled, "Enabled", "Disabled")}</td>
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
              <h2 className={styles.sectionTitle}>Agents</h2>
              <p className={styles.sectionCopy}>轮询在线态和最近心跳。</p>
            </div>
            <Link className={styles.secondaryButton} to="/agents">
              Open Agents
            </Link>
          </div>
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Online</th>
                  <th>Enabled</th>
                  <th>Heartbeat</th>
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
                        label={boolLabel(agent.online, "Online", "Offline")}
                        tone={agent.online ? "success" : "danger"}
                      />
                    </td>
                    <td>{boolLabel(agent.enabled, "Enabled", "Disabled")}</td>
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
              <h2 className={styles.sectionTitle}>Recent Releases</h2>
              <p className={styles.sectionCopy}>排队、部署、切流和回滚都在这里看。</p>
            </div>
            <Link className={styles.secondaryButton} to="/releases">
              Open Releases
            </Link>
          </div>
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>Release</th>
                  <th>Status</th>
                  <th>Image</th>
                  <th>Created</th>
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
                    <td>{releaseStatusLabel(release.status)}</td>
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
