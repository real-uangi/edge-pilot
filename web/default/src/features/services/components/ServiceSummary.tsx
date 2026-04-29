import { formatDateTime, slotLabel, boolLabel } from "../../../shared/lib/format";
import type { ServiceRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface ServiceSummaryProps {
  service: ServiceRecord;
}

export function ServiceSummary({ service }: ServiceSummaryProps) {
  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>运行摘要</h2>
        </div>
      </div>
      <div className={styles.keyValueGrid}>
        <div className={styles.keyValue}>
          <span className={styles.key}>当前槽位</span>
          <span className={styles.value}>{slotLabel(service.currentLiveSlot)}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>调度端口</span>
          <span className={styles.value}>{service.schedulerSdkPort || "-"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>执行器组</span>
          <span className={styles.value}>{service.schedulerExecutorGroup || "-"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>Docker 探活</span>
          <span className={styles.value}>{boolLabel(service.dockerHealthCheck)}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>CPU 限制</span>
          <span className={styles.value}>{service.cpuLimitCores > 0 ? `${service.cpuLimitCores} 核` : "不限"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>内存限制</span>
          <span className={styles.value}>{service.memoryLimitMB > 0 ? `${service.memoryLimitMB} MB` : "不限"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>预热窗口</span>
          <span className={styles.value}>{service.httpTimeoutSecond}s</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>启动宽限</span>
          <span className={styles.value}>{service.startupGraceSecond}s</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>探测节奏</span>
          <span className={styles.value}>
            {service.httpProbeIntervalSecond}s / timeout {service.httpProbeTimeoutSecond}s
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>连续成功</span>
          <span className={styles.value}>{service.httpSuccessThreshold} 次</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>探活 Header</span>
          <span className={styles.value}>{Object.keys(service.httpHealthHeaders ?? {}).length} 项</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>更新时间</span>
          <span className={styles.value}>{formatDateTime(service.updatedAt)}</span>
        </div>
      </div>
    </section>
  );
}
