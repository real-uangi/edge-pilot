import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime, slotLabel } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { StatusPill } from "../../../shared/components/StatusPill";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { servicesApi } from "../api";
import { agentsApi } from "../../agents/api";
import styles from "../../../styles/admin.module.css";

export function ServicesPage() {
  const [search, setSearch] = useState("");
  const [stateFilter, setStateFilter] = useState("");
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: servicesApi.list,
  });
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: agentsApi.list,
  });
  const agentsByID = new Map(
    (agentsQuery.data ?? []).map((agent) => [agent.id, agent] as const),
  );
  const term = search.trim().toLowerCase();
  const filteredServices = (servicesQuery.data ?? []).filter(
    (service) =>
      (!stateFilter ||
        (stateFilter === "enabled") === Boolean(service.enabled)) &&
      (!term ||
        [
          service.name,
          service.serviceKey,
          service.routeHost,
          service.imageRepo,
        ].some((value) => value.toLowerCase().includes(term))),
  );
  const clearFilters = () => {
    setSearch("");
    setStateFilter("");
  };

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div className={styles.pageTitleGroup}>
          <h1 className={styles.sectionTitle}>服务</h1>
          <span className={styles.resultCount}>
            {servicesQuery.data?.length ?? 0} 个服务
          </span>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新"
            pending={servicesQuery.isFetching}
            onClick={() => void servicesQuery.refetch()}
            variant="ghost"
          />
          <Link className={styles.primaryButton} to="/services/new">
            新建服务
          </Link>
        </div>
      </header>
      <QueryRefreshNotice
        query={servicesQuery}
        onRetry={() => void servicesQuery.refetch()}
      />
      <section className={styles.sectionCard} aria-label="服务列表">
        <div className={styles.filterBar}>
          <label className={styles.filterField}>
            <span className={styles.visuallyHidden}>搜索服务</span>
            <input
              className={styles.input}
              type="search"
              placeholder="搜索名称、标识、域名或镜像"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <label className={styles.filterField}>
            <span className={styles.visuallyHidden}>服务状态</span>
            <select
              className={styles.select}
              value={stateFilter}
              onChange={(event) => setStateFilter(event.target.value)}
            >
              <option value="">全部状态</option>
              <option value="enabled">启用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <span className={styles.resultCount} role="status">
            {filteredServices.length} / {servicesQuery.data?.length ?? 0}
          </span>
        </div>
        {servicesQuery.isPending ? (
          <LoadingState title="正在加载服务" />
        ) : servicesQuery.isError && !servicesQuery.data ? (
          <ErrorState
            title="服务加载失败"
            message={getErrorMessage(servicesQuery.error)}
            onRetry={() => servicesQuery.refetch()}
          />
        ) : !servicesQuery.data?.length ? (
          <EmptyState
            title="暂无服务"
            message="创建服务后即可配置路由和发布镜像。"
          />
        ) : !filteredServices.length ? (
          <div className={styles.emptyActions}>
            <EmptyState title="没有匹配的服务" />
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
                  <th>服务</th>
                  <th>节点</th>
                  <th>路由</th>
                  <th>当前槽位</th>
                  <th>状态</th>
                  <th>更新时间</th>
                </tr>
              </thead>
              <tbody>
                {filteredServices.map((service) => (
                  <tr key={service.id}>
                    <td data-label="服务">
                      <Link
                        className={styles.cellPrimary}
                        to={`/services/${service.id}`}
                      >
                        {service.name}
                      </Link>
                      <span
                        className={`${styles.cellSecondary} ${styles.mono}`}
                      >
                        {service.serviceKey}
                      </span>
                    </td>
                    <td data-label="节点">
                      <AgentLabel
                        id={service.agentId}
                        hostname={agentsByID.get(service.agentId)?.hostname}
                        ip={agentsByID.get(service.agentId)?.ip}
                      />
                    </td>
                    <td data-label="路由">
                      <span className={styles.technicalText}>
                        {service.routeHost + service.routePathPrefix}
                      </span>
                      {service.routeHosts.length > 1 ? (
                        <span className={styles.cellSecondary}>
                          共 {service.routeHosts.length} 个域名
                        </span>
                      ) : null}
                    </td>
                    <td data-label="当前槽位">
                      {slotLabel(service.currentLiveSlot)}
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
        )}
      </section>
    </div>
  );
}
