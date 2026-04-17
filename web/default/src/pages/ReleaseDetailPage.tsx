import { useEffect, useRef } from "react";
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
import { ActionButton } from "../components/ActionButton";
import { AgentLabel } from "../components/AgentLabel";
import { StatusPill } from "../components/StatusPill";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../components/StateBlocks";
import styles from "../styles/admin.module.css";

function confirmAction(message: string, action: () => void) {
  if (window.confirm(message)) {
    action();
  }
}

function cleanupLabel(value: boolean | null) {
  if (value == null) {
    return "—";
  }
  return value ? "已清理" : "未清理";
}

type ReleaseTaskLogInfo = {
  lastStep: string;
  dockerHealth: string;
  cleanupCompleted: boolean | null;
  lastError: string;
  failureLogs: string;
};

function TaskLogDetails({ task }: { task: ReleaseTaskLogInfo }) {
  const detailsRef = useRef<HTMLDetailsElement | null>(null);
  const logRef = useRef<HTMLPreElement | null>(null);

  const scrollLogToBottom = () => {
    if (!logRef.current) {
      return;
    }
    logRef.current.scrollTop = logRef.current.scrollHeight;
  };

  useEffect(() => {
    if (detailsRef.current?.open) {
      scrollLogToBottom();
    }
  }, [task.failureLogs]);

  return (
    <details
      className={styles.logCard}
      ref={detailsRef}
      onToggle={() => {
        if (detailsRef.current?.open) {
          scrollLogToBottom();
        }
      }}
    >
      <summary className={styles.logSummary}>查看详情和日志</summary>
      <div className={styles.logMeta}>
        <span>阶段：{task.lastStep || "—"}</span>
        <span>Docker 状态：{task.dockerHealth || "—"}</span>
        <span>清理：{cleanupLabel(task.cleanupCompleted)}</span>
        <span>错误：{task.lastError || "—"}</span>
      </div>
      {task.failureLogs ? (
        <pre className={styles.logBlock} ref={logRef}>
          {task.failureLogs}
        </pre>
      ) : (
        <div className={styles.logEmpty}>暂无失败日志</div>
      )}
    </details>
  );
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
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: api.listAgents,
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
  const retryMutation = useMutation({
    mutationFn: () => api.retryRelease(id!),
    onSuccess: invalidate,
  });

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
    startMutation.error ?? skipMutation.error ?? confirmMutation.error ?? rollbackMutation.error ?? retryMutation.error,
  );

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>发布详情</h1>
          <p className={styles.sectionCopy}>{release.id}</p>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton label="返回" onClick={() => navigate("/releases")} />
          <ActionButton label="刷新" onClick={() => detailQuery.refetch()} />
          <ActionButton
            label="开始"
            pending={startMutation.isPending}
            variant="primary"
            disabled={release.status !== 1}
            onClick={() => confirmAction("确认开始这个发布单？", () => startMutation.mutate())}
          />
          <ActionButton
            label="跳过"
            pending={skipMutation.isPending}
            variant="ghost"
            disabled={release.status !== 1}
            onClick={() => confirmAction("确认跳过这个发布单？", () => skipMutation.mutate())}
          />
          <ActionButton
            label="确认切流"
            pending={confirmMutation.isPending}
            variant="primary"
            disabled={release.status !== 4}
            onClick={() => confirmAction("确认执行切流？", () => confirmMutation.mutate())}
          />
          <ActionButton
            label="回滚"
            pending={rollbackMutation.isPending}
            variant="danger"
            disabled={[1, 9].includes(release.status)}
            onClick={() => confirmAction("确认回滚这个发布单？", () => rollbackMutation.mutate())}
          />
          <ActionButton
            label="重试"
            pending={retryMutation.isPending}
            variant="ghost"
            disabled={release.status !== 7}
            onClick={() => confirmAction("确认重试这个发布单？", () => retryMutation.mutate())}
          />
        </div>
      </section>

      {[
        startMutation.isError,
        skipMutation.isError,
        confirmMutation.isError,
        rollbackMutation.isError,
        retryMutation.isError,
      ].some(Boolean) ? (
        <InlineNotice message={actionError} tone="error" />
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
            <span className={styles.value}>
              <AgentLabel id={release.agentId} hostname={agent?.hostname} ip={agent?.ip} />
            </span>
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
          </div>
        </div>
        {!tasks.length ? (
          <EmptyState title="暂无任务" message="该发布单还未生成执行任务。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>任务</th>
                  <th>状态</th>
                  <th>派发时间</th>
                  <th>开始时间</th>
                  <th>完成时间</th>
                </tr>
              </thead>
              <tbody>
                {tasks.map((task) => (
                  [
                    <tr key={task.id}>
                      <td>{taskTypeLabel(task.type)}</td>
                      <td>
                        <StatusPill label={taskStatusLabel(task.status)} tone={taskStatusTone(task.status)} />
                      </td>
                      <td>{formatDateTime(task.dispatchedAt)}</td>
                      <td>{formatDateTime(task.startedAt)}</td>
                      <td>{formatDateTime(task.completedAt)}</td>
                    </tr>,
                    <tr className={styles.timelineDetailRow} key={task.id + "-details"}>
                      <td className={styles.timelineDetailCell} colSpan={5}>
                        <TaskLogDetails task={task} />
                      </td>
                    </tr>,
                  ]
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
