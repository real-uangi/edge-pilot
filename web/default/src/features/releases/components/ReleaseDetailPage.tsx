import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { useDialog } from "../../../shared/components/DialogProvider";
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { releasesApi } from "../api";
import { agentsApi } from "../../agents/api";
import { ReleaseInfo } from "./ReleaseInfo";
import { ReleaseActions } from "./ReleaseActions";
import { TrafficControl } from "./TrafficControl";
import { TaskTimeline } from "./TaskTimeline";
import styles from "../../../styles/admin.module.css";

export function ReleaseDetailPage() {
  const dialog = useDialog();
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [actionError, setActionError] = useState<string | null>(null);
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
  const invalidate = async (targetId: string) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["release", targetId] }),
      queryClient.invalidateQueries({ queryKey: ["releases"] }),
      queryClient.invalidateQueries({ queryKey: ["overview"] }),
    ]);
  };
  const onActionError = (error: unknown) =>
    setActionError(getErrorMessage(error));
  const startMutation = useMutation({
    mutationFn: (targetId: string) => releasesApi.start(targetId),
    onSuccess: (_, targetId) => invalidate(targetId),
    onError: onActionError,
  });
  const skipMutation = useMutation({
    mutationFn: (targetId: string) => releasesApi.skip(targetId),
    onSuccess: (_, targetId) => invalidate(targetId),
    onError: onActionError,
  });
  const confirmMutation = useMutation({
    mutationFn: (targetId: string) => releasesApi.confirmSwitch(targetId),
    onSuccess: (_, targetId) => invalidate(targetId),
    onError: onActionError,
  });
  const rollbackMutation = useMutation({
    mutationFn: (targetId: string) => releasesApi.rollback(targetId),
    onSuccess: (_, targetId) => invalidate(targetId),
    onError: onActionError,
  });
  const retryMutation = useMutation({
    mutationFn: (targetId: string) => releasesApi.retry(targetId),
    onSuccess: (_, targetId) => invalidate(targetId),
    onError: onActionError,
  });
  const trafficMutation = useMutation({
    mutationFn: ({ id: targetId, percent }: { id: string; percent: number }) =>
      releasesApi.setTraffic(targetId, percent),
    onSuccess: (_, { id: targetId }) => invalidate(targetId),
    onError: onActionError,
  });
  const busy =
    startMutation.isPending ||
    skipMutation.isPending ||
    confirmMutation.isPending ||
    rollbackMutation.isPending ||
    retryMutation.isPending ||
    trafficMutation.isPending;

  const confirmAction = async (
    targetId: string,
    title: string,
    message: string,
    action: (targetId: string) => void,
    danger = false,
  ) => {
    const confirmed = await dialog.confirm({
      title,
      message,
      confirmText: title,
      cancelText: "取消",
      danger,
    });
    if (confirmed) {
      setActionError(null);
      action(targetId);
    }
  };

  if (detailQuery.isPending)
    return (
      <div className={styles.page}>
        <LoadingState title="正在加载发布单" />
      </div>
    );
  if (detailQuery.isError && !detailQuery.data)
    return (
      <div className={styles.page}>
        <ErrorState
          title="发布单加载失败"
          message={getErrorMessage(detailQuery.error)}
          onRetry={() => detailQuery.refetch()}
        />
      </div>
    );
  if (!detailQuery.data)
    return (
      <div className={styles.page}>
        <EmptyState
          title="发布单不存在"
          actionLabel="返回发布"
          onRetry={() => navigate("/releases")}
        />
      </div>
    );

  const { release, tasks } = detailQuery.data;
  const agent = agentsQuery.data?.find((item) => item.id === release.agentId);
  const target = `${release.imageRepo}:${release.imageTag}`;
  const object = `${target}（发布单 ${release.id.slice(0, 8)}）`;

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div className={styles.pageTitleGroup}>
          <h1 className={styles.sectionTitle}>发布详情</h1>
          <p className={`${styles.sectionCopy} ${styles.mono}`}>{release.id}</p>
        </div>
        <ReleaseActions
          release={release}
          busy={busy}
          refreshPending={detailQuery.isFetching}
          startPending={startMutation.isPending}
          skipPending={skipMutation.isPending}
          confirmPending={confirmMutation.isPending}
          rollbackPending={rollbackMutation.isPending}
          retryPending={retryMutation.isPending}
          onStart={() =>
            void confirmAction(
              release.id,
              "开始部署",
              `部署 ${object}？`,
              startMutation.mutate,
            )
          }
          onSkip={() =>
            void confirmAction(
              release.id,
              "跳过发布",
              `跳过 ${object}？`,
              skipMutation.mutate,
            )
          }
          onConfirm={() =>
            void confirmAction(
              release.id,
              "确认切流",
              `将 ${object} 切换为正式版本？`,
              confirmMutation.mutate,
            )
          }
          onRollback={() =>
            void confirmAction(
              release.id,
              "确认回滚",
              `回滚 ${object}，流量将切回原版本。`,
              rollbackMutation.mutate,
              true,
            )
          }
          onRetry={() =>
            void confirmAction(
              release.id,
              "重试部署",
              `重新部署 ${object}？`,
              retryMutation.mutate,
            )
          }
          onBack={() => navigate("/releases")}
          onRefresh={() => void detailQuery.refetch()}
        />
      </header>
      <QueryRefreshNotice
        query={detailQuery}
        onRetry={() => void detailQuery.refetch()}
      />
      {actionError ? <InlineNotice message={actionError} tone="error" /> : null}
      <ReleaseInfo release={release} agent={agent} />
      {release.status === 4 ? (
        <TrafficControl
          key={release.id}
          release={release}
          disabled={busy}
          setTrafficPending={trafficMutation.isPending}
          onSetTraffic={(percent) =>
            void confirmAction(
              release.id,
              "应用流量比例",
              `将 ${object} 的流量比例设为 ${percent}%？`,
              (targetId) => trafficMutation.mutate({ id: targetId, percent }),
            )
          }
        />
      ) : null}
      <TaskTimeline tasks={tasks} audits={detailQuery.data.audits || []} />
      {detailQuery.data.releaseNotesAggregate?.length > 0 ? (
        <section className={styles.sectionCard}>
          <h2 className={styles.sectionSubtitle}>发布变更</h2>
          <div className={styles.notesList}>
            {detailQuery.data.releaseNotesAggregate.map((item) => (
              <div key={item.id} className={styles.notesItem}>
                <div className={styles.notesHeader}>
                  <span className={styles.notesTag}>{item.imageTag}</span>
                  <span className={styles.notesTime}>
                    {formatDateTime(item.createdAt)}
                  </span>
                </div>
                <p className={styles.notesBody}>{item.releaseNotes || "—"}</p>
              </div>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
