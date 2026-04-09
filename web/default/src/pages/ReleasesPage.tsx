import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { formatDateTime, releaseStatusLabel, releaseStatusTone, slotLabel } from "../lib/format";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function ReleasesPage() {
  const releasesQuery = useQuery({
    queryKey: ["releases"],
    queryFn: api.listReleases,
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>发布</h1>
        </div>
        <button className={styles.secondaryButton} onClick={() => releasesQuery.refetch()} type="button">
          刷新
        </button>
      </section>
      <section className={styles.sectionCard}>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>发布单</th>
                <th>状态</th>
                <th>镜像</th>
                <th>节点</th>
                <th>目标槽位</th>
                <th>队列</th>
                <th>创建时间</th>
              </tr>
            </thead>
            <tbody>
              {releasesQuery.data?.map((release) => (
                <tr key={release.id}>
                  <td>
                    <Link className={styles.tableLink} to={`/releases/${release.id}`}>
                      {release.id.slice(0, 8)}
                    </Link>
                  </td>
                  <td>
                    <StatusPill
                      label={releaseStatusLabel(release.status)}
                      tone={releaseStatusTone(release.status, release.isActive)}
                    />
                  </td>
                  <td>{release.imageRepo + ":" + release.imageTag}</td>
                  <td>{release.agentId}</td>
                  <td>{slotLabel(release.targetSlot)}</td>
                  <td>{release.queuePosition}</td>
                  <td>{formatDateTime(release.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
