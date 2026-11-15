import { Link } from "react-router-dom";
import {
  formatDateTime,
  releaseStatusLabel,
  releaseStatusTone,
  slotLabel,
} from "../../../shared/lib/format";
import { AgentLabel } from "../../../shared/components/AgentLabel";
import { StatusPill } from "../../../shared/components/StatusPill";
import type { ReleaseRecord } from "../types";
import type { AgentRecord } from "../../agents/types";
import styles from "../../../styles/admin.module.css";

export function ReleaseInfo({
  release,
  agent,
}: {
  release: ReleaseRecord;
  agent?: AgentRecord;
}) {
  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div className={styles.buttonRow}>
          <StatusPill
            label={releaseStatusLabel(release.status)}
            tone={releaseStatusTone(release.status, release.isActive)}
          />
          {release.switchConfirmed ? (
            <span className={styles.successText}>已确认切流</span>
          ) : null}
        </div>
        <Link
          className={styles.tableLink}
          to={`/services/${release.serviceId}`}
        >
          查看服务
        </Link>
      </div>
      <div className={styles.keyValueGrid}>
        <div className={styles.keyValue}>
          <span className={styles.key}>目标镜像</span>
          <span className={`${styles.value} ${styles.mono}`}>
            {release.imageRepo}:{release.imageTag}
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>当前流量比例</span>
          <span className={`${styles.value} ${styles.mono}`}>
            {release.trafficPercent}%
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>部署节点</span>
          <span className={styles.value}>
            <AgentLabel
              id={release.agentId}
              hostname={agent?.hostname}
              ip={agent?.ip}
            />
          </span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>目标槽位</span>
          <span className={styles.value}>{slotLabel(release.targetSlot)}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>触发人</span>
          <span className={styles.value}>{release.triggeredBy || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>创建时间</span>
          <span className={styles.value}>
            {formatDateTime(release.createdAt)}
          </span>
        </div>
      </div>
      {release.releaseNotes ? (
        <p className={styles.notesBody}>{release.releaseNotes}</p>
      ) : null}
    </section>
  );
}
