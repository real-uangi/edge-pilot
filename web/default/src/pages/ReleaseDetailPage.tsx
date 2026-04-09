import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { api, getErrorMessage } from "../lib/api";
import {
  formatDateTime,
  releaseStatusLabel,
  releaseStatusTone,
  slotLabel,
  taskStatusLabel,
  taskStatusTone,
  taskTypeLabel,
} from "../lib/format";
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
    return <div className={styles.page}>正在加载发布单…</div>;
  }
  if (!detailQuery.data) {
    return <div className={styles.page}>发布单不存在。</div>;
  }

  const { release, tasks } = detailQuery.data;
  const actionError =
    getErrorMessage(startMutation.error ?? skipMutation.error ?? confirmMutation.error ?? rollbackMutation.error);

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>发布详情</h1>
          <p className={styles.sectionCopy}>{release.id}</p>
        </div>
        <div className={styles.buttonRow}>
          <button className={styles.secondaryButton} onClick={() => navigate("/releases")} type="button">
            返回
          </button>
          <button className={styles.secondaryButton} onClick={() => detailQuery.refetch()} type="button">
            刷新
          </button>
          <button
            className={styles.primaryButton}
            disabled={release.status !== 1 || startMutation.isPending}
            onClick={() => confirmAction("确认开始这个发布单？", () => startMutation.mutate())}
            type="button"
          >
            开始
          </button>
          <button
            className={styles.ghostButton}
            disabled={release.status !== 1 || skipMutation.isPending}
            onClick={() => confirmAction("确认跳过这个发布单？", () => skipMutation.mutate())}
            type="button"
          >
            跳过
          </button>
          <button
            className={styles.primaryButton}
            disabled={release.status !== 4 || confirmMutation.isPending}
            onClick={() => confirmAction("确认执行切流？", () => confirmMutation.mutate())}
            type="button"
          >
            确认切流
          </button>
          <button
            className={styles.dangerButton}
            disabled={[1, 9].includes(release.status) || rollbackMutation.isPending}
            onClick={() => confirmAction("确认回滚这个发布单？", () => rollbackMutation.mutate())}
            type="button"
          >
            回滚
          </button>
        </div>
      </section>

      {[startMutation.isError, skipMutation.isError, confirmMutation.isError, rollbackMutation.isError].some(Boolean) ? (
        <div className={styles.error}>{actionError}</div>
      ) : null}

      <section className={styles.sectionCard}>
        <div className={styles.keyValueGrid}>
          <div className={styles.keyValue}>
            <span className={styles.key}>状态</span>
            <span className={styles.value}>{releaseStatusLabel(release.status)}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>镜像</span>
            <span className={styles.value}>{release.imageRepo + ":" + release.imageTag}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>节点</span>
            <span className={styles.value}>{release.agentId}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>目标槽位</span>
            <span className={styles.value}>{slotLabel(release.targetSlot)}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>前一槽位</span>
            <span className={styles.value}>{slotLabel(release.previousLiveSlot)}</span>
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

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>任务时间线</h2>
            <p className={styles.sectionCopy}>轮询任务状态直到发布结束。</p>
          </div>
        </div>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>任务</th>
                <th>状态</th>
                <th>错误</th>
                <th>派发时间</th>
                <th>开始时间</th>
                <th>完成时间</th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((task) => (
                <tr key={task.id}>
                  <td>{taskTypeLabel(task.type)}</td>
                  <td>
                    <StatusPill
                      label={taskStatusLabel(task.status)}
                      tone={taskStatusTone(task.status)}
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
