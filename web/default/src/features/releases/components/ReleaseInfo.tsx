import { formatDateTime, releaseStatusLabel, releaseStatusTone } from "../../../shared/lib/format";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { StatusPill } from "../../../shared/components/StatusPill";
import type { ReleaseRecord } from "../types";
import type { AgentRecord } from "../../agents/types";
import styles from "../../../styles/admin.module.css";

interface ReleaseInfoProps {
  release: ReleaseRecord;
  agent?: AgentRecord;
}

export function ReleaseInfo({ release, agent }: ReleaseInfoProps) {
  return (
    <section className={styles.sectionCard}>
      <div className={styles.keyValueGrid}>
        <div className={styles.keyValue}>
          <span className={styles.key}>状态</span>
          <span className={styles.value}>{releaseStatusLabel(release.status)}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>当前切流比例</span>
          <span className={styles.value}>{release.trafficPercent}%</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>镜像</span>
          <span className={styles.value}>{release.imageRepo + ":" + release.imageTag}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>更新说明</span>
          <span className={styles.value}>{release.releaseNotes || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>节点</span>
          <span className={styles.value}>
            <AgentLabel id={release.agentId} hostname={agent?.hostname} ip={agent?.ip} />
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>创建时间</span>
          <span className={styles.value}>{formatDateTime(release.createdAt)}</span>
        </div>
      </div>
      <StatusPill
        label={release.switchConfirmed ? "已确认切流" : "未确认切流"}
        tone={release.switchConfirmed ? "success" : releaseStatusTone(release.status, release.isActive)}
      />
    </section>
  );
}
