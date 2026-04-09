import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { api, getErrorMessage } from "../lib/api";
import { formatDateTime, releaseStatusLabel, slotLabel, taskStatusLabel, taskTypeLabel } from "../lib/format";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

function confirmAction(message: string, action: () => void) {
  if (window.confirm(message)) {
    action();
  }
}

export function ReleaseDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const detailQuery = useQuery({
    queryKey: ["release", id],
    queryFn: () => api.getRelease(id!),
    enabled: Boolean(id),
    refetchInterval: 5000,
  });

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["release", id] }),
      queryClient.invalidateQueries({ queryKey: ["releases"] }),
      queryClient.invalidateQueries({ queryKey: ["overview"] }),
    ]);
  };

  const startMutation = useMutation({
    mutationFn: () => api.startRelease(id!),
    onSuccess: invalidate,
  });
  const skipMutation = useMutation({
    mutationFn: () => api.skipRelease(id!),
    onSuccess: invalidate,
  });
  const confirmMutation = useMutation({
    mutationFn: () => api.confirmSwitch(id!),
    onSuccess: invalidate,
  });
  const rollbackMutation = useMutation({
    mutationFn: () => api.rollbackRelease(id!),
    onSuccess: invalidate,
  });

  if (detailQuery.isPending) {
    return <div className={styles.page}>Loading release…</div>;
  }
  if (!detailQuery.data) {
    return <div className={styles.page}>Release not found.</div>;
  }

  const { release, tasks } = detailQuery.data;
  const actionError =
    getErrorMessage(startMutation.error ?? skipMutation.error ?? confirmMutation.error ?? rollbackMutation.error);

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>Release Detail</h1>
          <p className={styles.sectionCopy}>{release.id}</p>
        </div>
        <div className={styles.buttonRow}>
          <button className={styles.secondaryButton} onClick={() => navigate("/releases")} type="button">
            Back
          </button>
          <button className={styles.secondaryButton} onClick={() => detailQuery.refetch()} type="button">
            Refresh
          </button>
          <button
            className={styles.primaryButton}
            disabled={release.status !== 1 || startMutation.isPending}
            onClick={() => confirmAction("Start this release?", () => startMutation.mutate())}
            type="button"
          >
            Start
          </button>
          <button
            className={styles.ghostButton}
            disabled={release.status !== 1 || skipMutation.isPending}
            onClick={() => confirmAction("Skip this release?", () => skipMutation.mutate())}
            type="button"
          >
            Skip
          </button>
          <button
            className={styles.primaryButton}
            disabled={release.status !== 4 || confirmMutation.isPending}
            onClick={() => confirmAction("Confirm traffic switch?", () => confirmMutation.mutate())}
            type="button"
          >
            Confirm Switch
          </button>
          <button
            className={styles.dangerButton}
            disabled={[1, 9].includes(release.status) || rollbackMutation.isPending}
            onClick={() => confirmAction("Rollback this release?", () => rollbackMutation.mutate())}
            type="button"
          >
            Rollback
          </button>
        </div>
      </section>

      {[startMutation.isError, skipMutation.isError, confirmMutation.isError, rollbackMutation.isError].some(Boolean) ? (
        <div className={styles.error}>{actionError}</div>
      ) : null}

      <section className={styles.sectionCard}>
        <div className={styles.keyValueGrid}>
          <div className={styles.keyValue}>
            <span className={styles.key}>Status</span>
            <span className={styles.value}>{releaseStatusLabel(release.status)}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Image</span>
            <span className={styles.value}>{release.imageRepo + ":" + release.imageTag}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Agent</span>
            <span className={styles.value}>{release.agentId}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Target Slot</span>
            <span className={styles.value}>{slotLabel(release.targetSlot)}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Previous Slot</span>
            <span className={styles.value}>{slotLabel(release.previousLiveSlot)}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Created</span>
            <span className={styles.value}>{formatDateTime(release.createdAt)}</span>
          </div>
        </div>
        <StatusPill
          label={release.switchConfirmed ? "Switch Confirmed" : "Switch Not Confirmed"}
          tone={release.switchConfirmed ? "success" : "warning"}
        />
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>Task Timeline</h2>
            <p className={styles.sectionCopy}>轮询任务状态直到发布结束。</p>
          </div>
        </div>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>Task</th>
                <th>Status</th>
                <th>Error</th>
                <th>Dispatched</th>
                <th>Started</th>
                <th>Completed</th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((task) => (
                <tr key={task.id}>
                  <td>{taskTypeLabel(task.type)}</td>
                  <td>
                    <StatusPill
                      label={taskStatusLabel(task.status)}
                      tone={task.status === 4 ? "success" : task.status === 5 || task.status === 6 ? "danger" : "default"}
                    />
                  </td>
                  <td>{task.lastError || "—"}</td>
                  <td>{formatDateTime(task.dispatchedAt)}</td>
                  <td>{formatDateTime(task.startedAt)}</td>
                  <td>{formatDateTime(task.completedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
