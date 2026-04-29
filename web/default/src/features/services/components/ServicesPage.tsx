import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime, slotLabel } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { servicesApi } from "../api";
import { agentsApi } from "../../agents/api";
import styles from "../../../styles/admin.module.css";

export function ServicesPage() {
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: servicesApi.list,
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
          <h1 className={styles.sectionTitle}>服务</h1>
          <p className={styles.sectionCopy}>服务配置、路由和探活参数统一在此维护。</p>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton label="刷新" onClick={() => servicesQuery.refetch()} />
          <Link className={styles.primaryButton} to="/services/new">
            新建服务
          </Link>
        </div>
      </section>

      <section className={styles.sectionCard}>
        {servicesQuery.isPending ? (
          <LoadingState title="正在加载服务列表" message="正在拉取服务配置和运行摘要。" />
        ) : servicesQuery.isError ? (
          <ErrorState
            title="服务列表加载失败"
            message={getErrorMessage(servicesQuery.error)}
            onRetry={() => servicesQuery.refetch()}
          />
        ) : !servicesQuery.data?.length ? (
          <EmptyState
            title="暂无服务"
            message="当前还没有服务配置，可先创建一个服务。"
          />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>名称</th>
                  <th>标识</th>
                  <th>节点</th>
                  <th>路由</th>
                  <th>当前槽位</th>
                  <th>启用</th>
                  <th>更新时间</th>
                </tr>
              </thead>
              <tbody>
                {servicesQuery.data.map((service) => (
                  <tr key={service.id}>
                    <td>
                      <Link className={styles.tableLink} to={`/services/${service.id}`}>
                        {service.name}
                      </Link>
                    </td>
                    <td>{service.serviceKey}</td>
                    <td>
                      <AgentLabel
                        id={service.agentId}
                        hostname={agentsByID.get(service.agentId)?.hostname}
                        ip={agentsByID.get(service.agentId)?.ip}
                      />
                    </td>
                    <td>{service.routeHost + service.routePathPrefix}</td>
                    <td>{slotLabel(service.currentLiveSlot)}</td>
                    <td>
                      <StatusPill
                        label={service.enabled ? "启用" : "停用"}
                        tone={service.enabled ? "success" : "danger"}
                      />
                    </td>
                    <td>{formatDateTime(service.updatedAt)}</td>
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
