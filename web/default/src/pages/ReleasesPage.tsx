import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { formatDateTime, releaseStatusLabel, slotLabel } from "../lib/format";
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
          <h1 className={styles.sectionTitle}>Releases</h1>
          <p className={styles.sectionCopy}>查看排队、部署和切流状态，深入操作放在详情页。</p>
        </div>
        <button className={styles.secondaryButton} onClick={() => releasesQuery.refetch()} type="button">
          Refresh
        </button>
      </section>
      <section className={styles.sectionCard}>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>Release</th>
                <th>Status</th>
                <th>Image</th>
                <th>Agent</th>
                <th>Target Slot</th>
                <th>Queue</th>
                <th>Created</th>
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
                      tone={release.isActive ? "warning" : release.status === 6 ? "success" : release.status === 7 ? "danger" : "default"}
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
