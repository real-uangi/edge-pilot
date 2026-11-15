import { formatDateTime, slotLabel } from "../../../shared/lib/format";
import type { ServiceRecord } from "../types";
import styles from "../../../styles/admin.module.css";

export function ServiceSummary({ service }: { service: ServiceRecord }) {
  return (
    <section className={styles.sectionCard}>
      <h2 className={styles.sectionSubtitle}>运行摘要</h2>
      <div className={styles.keyValueGrid}>
        <div className={styles.keyValue}>
          <span className={styles.key}>当前槽位</span>
          <span className={styles.value}>
            {slotLabel(service.currentLiveSlot)}
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>主路由</span>
          <span className={`${styles.value} ${styles.mono}`}>
            {service.routeHost}
            {service.routePathPrefix}
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>镜像仓库</span>
          <span className={`${styles.value} ${styles.mono}`}>
            {service.imageRepo}
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>CPU 限制</span>
          <span className={styles.value}>
            {service.cpuLimitCores > 0 ? `${service.cpuLimitCores} 核` : "不限"}
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>内存限制</span>
          <span className={styles.value}>
            {service.memoryLimitMB > 0 ? `${service.memoryLimitMB} MB` : "不限"}
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>配置更新时间</span>
          <span className={styles.value}>
            {formatDateTime(service.updatedAt)}
          </span>
        </div>
      </div>
    </section>
  );
}
