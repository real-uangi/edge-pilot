import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { useDialog } from "../../../shared/components/DialogProvider";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../../../shared/components/StateBlocks";
import { releasesApi } from "../api";
import { agentsApi } from "../../agents/api";
import { ReleaseInfo } from "./ReleaseInfo";
import { ReleaseActions } from "./ReleaseActions";
import { TrafficControl } from "./TrafficControl";
import { TaskTimeline } from "./TaskTimeline";
import styles from "../../../styles/admin.module.css";

function isConflictError(error: unknown) {
  if (!error || typeof error !== "object") {
    return false;
  }
  const status = (error as { status?: unknown }).status;
  return typeof status === "number" && status === 409;
}

function isTrafficGateError(error: unknown) {
  return isConflictError(error) && getErrorMessage(error).includes("1-99");
}

export function ReleaseDetailPage() {
  const dialog = useDialog();
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const detailQuery = useQuery({
    queryKey: ["release", id],
    queryFn: () => releasesApi.get(id!),
    enabled: Boolean(id),
    refetchInterval: 5000,
  });
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: agentsApi.list,
  });

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["release", id] }),
      queryClient.invalidateQueries({ queryKey: ["releases"] }),
      queryClient.invalidateQueries({ queryKey: ["overview"] }),
    ]);
  };

  const startMutation = useMutation({
    mutationFn: () => releasesApi.start(id!),
    onSuccess: invalidate,
    onError: (error) => {
      if (!isTrafficGateError(error)) {
        return;
      }
      void dialog.alert({
        title: "无法开始发布",
        message: getErrorMessage(error),
      });
    },
  });
  const skipMutation = useMutation({
    mutationFn: () => releasesApi.skip(id!),
    onSuccess: invalidate,
  });
  const confirmMutation = useMutation({
    mutationFn: () => releasesApi.confirmSwitch(id!),
    onSuccess: invalidate,
  });
  const rollbackMutation = useMutation({
    mutationFn: () => releasesApi.rollback(id!),
    onSuccess: invalidate,
  });
  const retryMutation = useMutation({
    mutationFn: () => releasesApi.retry(id!),
    onSuccess: invalidate,
  });
  const trafficMutation = useMutation({
    mutationFn: (percent: number) => releasesApi.setTraffic(id!, percent),
    onSuccess: invalidate,
  });

  const confirmAction = async (message: string, action: () => void) => {
    const confirmed = await dialog.confirm({
      message,
      confirmText: "确认",
      cancelText: "取消",
    });
    if (confirmed) {
      action();
    }
  };

  if (detailQuery.isPending) {
    return (
      <div className={styles.page}>
        <LoadingState title="正在加载发布单" message="正在同步任务时间线和节点状态。" />
      </div>
    );
  }

  if (detailQuery.isError) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="发布单加载失败"
          message={getErrorMessage(detailQuery.error)}
          onRetry={() => detailQuery.refetch()}
        />
      </div>
    );
  }

  if (!detailQuery.data) {
    return (
      <div className={styles.page}>
        <EmptyState title="发布单不存在" message="请返回发布列表后重新选择。" />
      </div>
    );
  }

  const { release, tasks } = detailQuery.data;
  const agent = agentsQuery.data?.find((item) => item.id === release.agentId);
  const actionError = getErrorMessage(
    startMutation.error ??
      skipMutation.error ??
      confirmMutation.error ??
      rollbackMutation.error ??
      retryMutation.error ??
      trafficMutation.error,
  );

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>发布详情</h1>
          <p className={styles.sectionCopy}>{release.id}</p>
        </div>
        <ReleaseActions
          release={release}
          startPending={startMutation.isPending}
          skipPending={skipMutation.isPending}
          confirmPending={confirmMutation.isPending || trafficMutation.isPending}
          rollbackPending={rollbackMutation.isPending || trafficMutation.isPending}
          retryPending={retryMutation.isPending}
          onStart={() => void confirmAction("确认开始这个发布单？", () => startMutation.mutate())}
          onSkip={() => void confirmAction("确认跳过这个发布单？", () => skipMutation.mutate())}
          onConfirm={() => void confirmAction("确认执行切流？", () => confirmMutation.mutate())}
          onRollback={() => void confirmAction("确认回滚这个发布单？", () => rollbackMutation.mutate())}
          onRetry={() => void confirmAction("确认重试这个发布单？", () => retryMutation.mutate())}
          onBack={() => navigate("/releases")}
          onRefresh={() => detailQuery.refetch()}
        />
      </section>

      {[
        startMutation.isError,
        skipMutation.isError,
        confirmMutation.isError,
        rollbackMutation.isError,
        retryMutation.isError,
        trafficMutation.isError,
      ].some(Boolean) ? (
        <InlineNotice message={actionError} tone="error" />
      ) : null}

      <ReleaseInfo release={release} agent={agent} />

      {detailQuery.data.releaseNotesAggregate?.length > 0 && (
        <section className={styles.sectionCard}>
          <h3 className={styles.sectionSubtitle}>本次发布变更汇总</h3>
          <div className={styles.notesList}>
            {detailQuery.data.releaseNotesAggregate.map((item) => (
              <div key={item.id} className={styles.notesItem}>
                <div className={styles.notesHeader}>
                  <span className={styles.notesTag}>{item.imageTag}</span>
                  <span className={styles.notesTime}>{formatDateTime(item.createdAt)}</span>
                </div>
                <p className={styles.notesBody}>{item.releaseNotes || "—"}</p>
              </div>
            ))}
          </div>
        </section>
      )}

      <TrafficControl
        release={release}
        setTrafficPending={trafficMutation.isPending}
        onSetTraffic={(percent) => void confirmAction(`确认设置切流比例为 ${percent}%？`, () => trafficMutation.mutate(percent))}
      />

      <TaskTimeline tasks={tasks} audits={detailQuery.data.audits || []} />
    </div>
  );
}
