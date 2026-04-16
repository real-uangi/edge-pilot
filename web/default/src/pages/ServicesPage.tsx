import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { formatDateTime, slotLabel } from "../lib/format";
import { AgentLabel } from "../components/AgentLabel";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function ServicesPage() {
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: api.listServices,
  });
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: api.listAgents,
  });
  const agentsByID = new Map((agentsQuery.data ?? []).map((agent) => [agent.id, agent] as const));

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>服务</h1>
        </div>
        <div className={styles.buttonRow}>
          <button className={styles.secondaryButton} onClick={() => servicesQuery.refetch()} type="button">
            刷新
          </button>
          <Link className={styles.primaryButton} to="/services/new">
            新建服务
          </Link>
        </div>
      </section>

      <section className={styles.sectionCard}>
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
              {servicesQuery.data?.map((service) => (
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
      </section>
    </div>
  );
}
