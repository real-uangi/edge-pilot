import { formatDateTime, boolLabel } from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import type { AgentRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface AgentInfoProps {
  agent: AgentRecord;
}

export function AgentInfo({ agent }: AgentInfoProps) {
  return (
    <section className={styles.sectionCard}>
      <div className={styles.keyValueGrid}>
        <div className={styles.keyValue}>
          <span className={styles.key}>节点 ID</span>
          <span className={styles.value}>{agent.id}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>在线状态</span>
          <span className={styles.value}>{boolLabel(agent.online, "在线", "离线")}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>启用状态</span>
          <span className={styles.value}>{boolLabel(agent.enabled, "启用", "停用")}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>主机名</span>
          <span className={styles.value}>{agent.hostname || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>IP</span>
          <span className={styles.value}>{agent.ip || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>版本</span>
          <span className={styles.value}>{agent.version || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>最近连接</span>
          <span className={styles.value}>{formatDateTime(agent.lastConnectedAt)}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>最近心跳</span>
          <span className={styles.value}>{formatDateTime(agent.lastHeartbeatAt)}</span>
        </div>
      </div>
      <StatusPill label={agent.lastError || "最近没有错误"} tone={agent.lastError ? "danger" : "success"} />
    </section>
  );
}
