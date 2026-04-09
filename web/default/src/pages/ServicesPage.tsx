import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { formatDateTime, slotLabel } from "../lib/format";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function ServicesPage() {
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: api.listServices,
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>Services</h1>
          <p className={styles.sectionCopy}>管理路由、探活、镜像仓库和容器暴露端口。</p>
        </div>
        <div className={styles.buttonRow}>
          <button className={styles.secondaryButton} onClick={() => servicesQuery.refetch()} type="button">
            Refresh
          </button>
          <Link className={styles.primaryButton} to="/services/new">
            New Service
          </Link>
        </div>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Key</th>
                <th>Agent</th>
                <th>Route</th>
                <th>Live Slot</th>
                <th>Enabled</th>
                <th>Updated</th>
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
                  <td>{service.agentId}</td>
                  <td>{service.routeHost + service.routePathPrefix}</td>
                  <td>{slotLabel(service.currentLiveSlot)}</td>
                  <td>
                    <StatusPill
                      label={service.enabled ? "Enabled" : "Disabled"}
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
