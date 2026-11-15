import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { ArrowUpRight, Box, Layers, Radio, Server } from "lucide-react";
import { getErrorMessage } from "../../../shared/lib/api-client";
import {
  boolLabel,
  formatDateTime,
  releaseStatusLabel,
  releaseStatusTone,
} from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import { ActionButton } from "../../../shared/components/ActionButton";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { dashboardApi } from "../api";
import styles from "../../../styles/admin.module.css";
import dashboard from "./DashboardPage.module.css";

export function DashboardPage() {
  const navigate = useNavigate();
  const query = useQuery({
    queryKey: ["overview"],
    queryFn: dashboardApi.overview,
    refetchInterval: 10000,
  });
  if (query.isPending) return <LoadingState title="正在加载总览" />;
  if (!query.data)
    return (
      <ErrorState
        title="总览加载失败"
        message={getErrorMessage(query.error)}
        onRetry={() => void query.refetch()}
      />
    );

  const { agents, services, recentReleases, activeInstances } = query.data;
  const servicesById = new Map(
    services.map((service) => [service.id, service]),
  );
  const agentsById = new Map(agents.map((agent) => [agent.id, agent]));
  const attention = recentReleases.filter(
    (release) => [1, 4, 7].includes(release.status) || release.isActive,
  );
  const sortedAgents = [...agents].sort(
    (a, b) => Number(a.online === true) - Number(b.online === true),
  );
  const metrics = [
    {
      label: "运行实例",
      value: activeInstances,
      meta: "受管容器",
      to: "/instances",
      Icon: Box,
    },
    {
      label: "在线节点",
      value: agents.filter((agent) => agent.online === true).length,
      meta: `共 ${agents.length} 个节点`,
      to: "/agents",
      Icon: Server,
    },
    {
      label: "启用服务",
      value: services.filter((service) => service.enabled).length,
      meta: `共 ${services.length} 个服务`,
      to: "/services",
      Icon: Layers,
    },
    {
      label: "近期活动发布",
      value: recentReleases.filter((release) => release.isActive).length,
      meta: `最近 ${recentReleases.length} 条发布记录`,
      to: "/releases",
      Icon: Radio,
    },
  ];

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div className={styles.pageTitleGroup}>
          <span className={styles.eyebrow}>WORKSPACE</span>
          <h1 className={styles.sectionTitle}>总览</h1>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新"
            pending={query.isFetching}
            onClick={() => void query.refetch()}
          />
          <Link className={styles.primaryButton} to="/services/new">
            新建服务
          </Link>
        </div>
      </header>
      <section className={styles.cardGrid} aria-label="运行摘要">
        {metrics.map(({ label, value, meta, to, Icon }) => (
          <Link to={to} className={dashboard.metric} key={label}>
            <div className={dashboard.metricTop}>
              <span>{label}</span>
              <Icon size={17} aria-hidden="true" />
            </div>
            <span className={dashboard.metricValue}>{value}</span>
            <div className={dashboard.metricBottom}>
              <span>{meta}</span>
              <ArrowUpRight size={15} aria-hidden="true" />
            </div>
          </Link>
        ))}
      </section>
      <QueryRefreshNotice query={query} onRetry={() => void query.refetch()} />
      <section className={dashboard.workGrid}>
        <div className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className={styles.sectionTitle}>发布工作台</h2>
              <p className={styles.sectionCopy}>
                近期发布中的待处理项与失败记录
              </p>
            </div>
            <Link className={styles.ghostButton} to="/releases">
              全部发布 <span aria-hidden="true">↗</span>
            </Link>
          </div>
          {attention.length ? (
            <div className={dashboard.releaseList}>
              {attention.slice(0, 6).map((release) => (
                <Link
                  className={dashboard.releaseRow}
                  to={`/releases/${release.id}`}
                  key={release.id}
                >
                  <div className={dashboard.releaseMain}>
                    <strong>
                      {servicesById.get(release.serviceId)?.name ||
                        release.imageRepo}
                    </strong>
                    <span className={styles.mono}>
                      {release.imageTag}{" "}
                      <span className={styles.muted}>
                        · {release.id.slice(0, 8)}
                      </span>
                    </span>
                  </div>
                  <div className={dashboard.releaseEnd}>
                    <StatusPill
                      label={releaseStatusLabel(release.status)}
                      tone={releaseStatusTone(release.status, release.isActive)}
                    />
                    <span className={styles.hint}>
                      {formatDateTime(release.updatedAt)}
                    </span>
                  </div>
                  <ArrowUpRight
                    size={16}
                    className={dashboard.rowArrow}
                    aria-hidden="true"
                  />
                </Link>
              ))}
            </div>
          ) : (
            <EmptyState
              title="近期没有待处理发布"
              message={
                recentReleases.length
                  ? "可前往发布列表查看已有记录。"
                  : "从服务详情选择镜像 Tag 创建发布单。"
              }
            />
          )}
        </div>
        <div className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>节点状态</h2>
            <Link className={styles.ghostButton} to="/agents">
              查看节点
            </Link>
          </div>
          {sortedAgents.length ? (
            <div className={dashboard.nodeList}>
              {sortedAgents.slice(0, 5).map((agent) => (
                <Link
                  to={`/agents/${agent.id}`}
                  className={dashboard.node}
                  key={agent.id}
                >
                  <div className={dashboard.nodeIdentity}>
                    <span className={dashboard.nodeIcon}>
                      <Server size={17} aria-hidden="true" />
                    </span>
                    <div>
                      <strong>{agent.hostname || agent.id.slice(0, 8)}</strong>
                      <span className={styles.mono}>{agent.ip || "—"}</span>
                    </div>
                  </div>
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
                </Link>
              ))}
            </div>
          ) : (
            <EmptyState
              title="尚未连接节点"
              actionLabel="创建节点"
              onRetry={() => {
                navigate("/agents");
              }}
            />
          )}
        </div>
      </section>
      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>服务目录</h2>
            <p className={styles.sectionCopy}>{services.length} 个服务</p>
          </div>
          <Link className={styles.ghostButton} to="/services">
            查看全部
          </Link>
        </div>
        {services.length ? (
          <div className={styles.tableWrap}>
            <table className={styles.responsiveTable}>
              <thead>
                <tr>
                  <th>服务</th>
                  <th>路由</th>
                  <th>节点</th>
                  <th>状态</th>
                  <th>更新时间</th>
                </tr>
              </thead>
              <tbody>
                {services.slice(0, 8).map((service) => (
                  <tr key={service.id}>
                    <td data-label="服务">
                      <Link
                        className={styles.tableLink}
                        to={`/services/${service.id}`}
                      >
                        {service.name}
                      </Link>
                      <div className={styles.hint}>{service.serviceKey}</div>
                    </td>
                    <td data-label="路由" className={styles.mono}>
                      {service.routeHost + service.routePathPrefix}
                    </td>
                    <td data-label="节点">
                      {agentsById.get(service.agentId)?.hostname ||
                        service.agentId.slice(0, 8)}
                    </td>
                    <td data-label="状态">
                      <StatusPill
                        label={service.enabled ? "启用" : "停用"}
                        tone={service.enabled ? "success" : "default"}
                      />
                    </td>
                    <td data-label="更新时间">
                      {formatDateTime(service.updatedAt)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState
            title="尚未创建服务"
            message="创建服务后，可在这里查看运行状态并发起发布。"
          />
        )}
      </section>
    </div>
  );
}
