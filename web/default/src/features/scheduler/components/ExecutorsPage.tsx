import { useState } from "react";
import {
  useMutation,
  useIsMutating,
  useMutationState,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime, boolLabel } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { CredentialResult } from "../../../shared/components/CredentialResult";
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { useDialog } from "../../../shared/components/DialogProvider";
import { Modal } from "../../../shared/components/Modal";
import { StatusPill } from "../../../shared/components/StatusPill";
import { schedulerApi } from "../api";
import type { SchedulerExecutorRecord } from "../types";
import { ExecutorForm } from "./ExecutorForm";
import styles from "../../../styles/admin.module.css";

type ExecutorAction = {
  executor: SchedulerExecutorRecord;
  action: "reset" | "toggle" | "delete";
};

export function ExecutorsPage() {
  const queryClient = useQueryClient();
  const dialog = useDialog();
  const [creating, setCreating] = useState(false);
  const createPending =
    useIsMutating({ mutationKey: ["scheduler", "create-executor"] }) > 0;
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [credential, setCredential] = useState<{
    id: string;
    token: string;
  } | null>(null);
  const [feedback, setFeedback] = useState<{
    message: string;
    error: boolean;
  } | null>(null);
  const executorsQuery = useQuery({
    queryKey: ["scheduler", "executors"],
    queryFn: schedulerApi.listExecutors,
    refetchInterval: 10000,
  });
  const actionMutation = useMutation({
    mutationKey: ["scheduler", "executor-action"],
    mutationFn: async ({ executor, action }: ExecutorAction) => {
      if (action === "reset") {
        const output = await schedulerApi.resetExecutorToken(executor.id);
        if (output.token) setCredential({ id: output.id, token: output.token });
      } else if (action === "delete")
        await schedulerApi.deleteExecutor(executor.id);
      else if (executor.enabled)
        await schedulerApi.disableExecutor(executor.id);
      else await schedulerApi.enableExecutor(executor.id);
    },
    onMutate: () => setFeedback(null),
    onSuccess: async (_, { executor, action }) => {
      setFeedback({
        message:
          action === "reset"
            ? `已重置「${executor.id}」的令牌`
            : action === "delete"
              ? `已删除「${executor.id}」`
              : `已${executor.enabled ? "停用" : "启用"}「${executor.id}」`,
        error: false,
      });
      await queryClient.invalidateQueries({
        queryKey: ["scheduler", "executors"],
      });
    },
    onError: (error, { executor }) =>
      setFeedback({
        message: `${executor.id}：${getErrorMessage(error)}`,
        error: true,
      }),
  });
  const pending = useMutationState({
    filters: {
      mutationKey: ["scheduler", "executor-action"],
      status: "pending",
    },
    select: (mutation) => mutation.state.variables as ExecutorAction,
  });
  const handleAction = async (
    executor: SchedulerExecutorRecord,
    action: ExecutorAction["action"],
  ) => {
    if (
      action !== "toggle" &&
      !(await dialog.confirm({
        title: action === "reset" ? "重置执行器令牌" : "删除执行器",
        message:
          action === "reset"
            ? `重置「${executor.id}」后，旧令牌立即失效，执行器需要重新连接。确认重置？`
            : `确认删除执行器「${executor.id}」？正在执行的任务可能受影响。`,
        confirmText: action === "reset" ? "重置令牌" : "删除执行器",
        danger: true,
      }))
    )
      return;
    actionMutation.mutate({ executor, action });
  };
  const query = search.trim().toLowerCase();
  const executors = (executorsQuery.data ?? []).filter(
    (executor) =>
      (!query ||
        [executor.id, executor.group].some((value) =>
          value.toLowerCase().includes(query),
        )) &&
      (status === "all" ||
        (status === "online"
          ? executor.online
          : status === "offline"
            ? !executor.online
            : executor.enabled === false)),
  );
  const tokenPending = pending.some((item) => item.action === "reset");

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <h1 className={styles.sectionTitle}>执行器</h1>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新"
            pending={executorsQuery.isFetching}
            onClick={() => void executorsQuery.refetch()}
          />
          <ActionButton
            label="新建执行器"
            disabled={Boolean(credential) || tokenPending || createPending}
            variant="primary"
            onClick={() => setCreating(true)}
          />
        </div>
      </header>
      {feedback ? (
        <InlineNotice
          message={feedback.message}
          tone={feedback.error ? "error" : "info"}
        />
      ) : null}
      {credential ? (
        <CredentialResult
          id={credential.id}
          token={credential.token}
          onClose={() => setCredential(null)}
        />
      ) : null}
      <section className={styles.sectionCard}>
        <div className={styles.listToolbar}>
          <label className={styles.searchField}>
            <span className={styles.visuallyHidden}>搜索执行器</span>
            <input
              className={styles.input}
              type="search"
              placeholder="搜索执行器 ID 或组"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <label className={styles.filterGroup}>
            <span className={styles.visuallyHidden}>执行器状态</span>
            <select
              className={styles.select}
              value={status}
              onChange={(event) => setStatus(event.target.value)}
            >
              <option value="all">全部状态</option>
              <option value="online">在线</option>
              <option value="offline">离线</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <span className={styles.resultCount} aria-live="polite">
            {executors.length} 个执行器
          </span>
        </div>
        <QueryRefreshNotice
          query={executorsQuery}
          onRetry={() => void executorsQuery.refetch()}
        />
        {executorsQuery.isPending ? (
          <LoadingState title="正在加载执行器" />
        ) : executorsQuery.isError && !executorsQuery.data ? (
          <ErrorState
            title="执行器加载失败"
            message={getErrorMessage(executorsQuery.error)}
            onRetry={() => void executorsQuery.refetch()}
          />
        ) : !executorsQuery.data?.length ? (
          <EmptyState
            title="暂无执行器"
            actionLabel="新建执行器"
            onRetry={() => setCreating(true)}
          />
        ) : !executors.length ? (
          <EmptyState
            title="没有匹配的执行器"
            actionLabel="清除筛选"
            onRetry={() => {
              setSearch("");
              setStatus("all");
            }}
          />
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.responsiveTable}>
              <thead>
                <tr>
                  <th>执行器 / 组</th>
                  <th>通道</th>
                  <th>活跃槽位</th>
                  <th>连接</th>
                  <th>状态</th>
                  <th>最近心跳</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {executors.map((executor) => {
                  const pendingAction = pending.find(
                    (item) => item.executor.id === executor.id,
                  )?.action;
                  return (
                    <tr key={executor.id}>
                      <td data-label="执行器">
                        <code>{executor.id}</code>
                        <div className={styles.muted}>{executor.group}</div>
                      </td>
                      <td data-label="通道">
                        {executor.channelMode === 2 ? "代理中继" : "直连"}
                      </td>
                      <td data-label="活跃槽位">
                        {executor.liveSlot === 1
                          ? "蓝槽"
                          : executor.liveSlot === 2
                            ? "绿槽"
                            : "—"}
                      </td>
                      <td data-label="连接">
                        <StatusPill
                          label={executor.online ? "在线" : "离线"}
                          tone={executor.online ? "success" : "warning"}
                        />
                      </td>
                      <td data-label="状态">
                        {boolLabel(executor.enabled, "启用", "停用")}
                      </td>
                      <td data-label="最近心跳">
                        {formatDateTime(executor.lastSeenAt)}
                      </td>
                      <td data-label="操作">
                        <div className={styles.buttonRow}>
                          <ActionButton
                            label={executor.enabled ? "停用" : "启用"}
                            pending={pendingAction === "toggle"}
                            disabled={Boolean(pendingAction)}
                            onClick={() =>
                              void handleAction(executor, "toggle")
                            }
                          />
                          <ActionButton
                            label="重置令牌"
                            variant="ghost"
                            pending={pendingAction === "reset"}
                            disabled={
                              Boolean(pendingAction) ||
                              Boolean(credential) ||
                              tokenPending
                            }
                            onClick={() => void handleAction(executor, "reset")}
                          />
                          <ActionButton
                            label="删除"
                            variant="danger"
                            pending={pendingAction === "delete"}
                            disabled={Boolean(pendingAction)}
                            onClick={() =>
                              void handleAction(executor, "delete")
                            }
                          />
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
      {creating ? (
        <Modal
          title="新建执行器"
          closeDisabled={createPending}
          onClose={() => setCreating(false)}
        >
          <ExecutorForm
            onCreated={(output) => {
              setCreating(false);
              if (output.token)
                setCredential({ id: output.id, token: output.token });
            }}
            onCancel={() => setCreating(false)}
          />
        </Modal>
      ) : null}
    </div>
  );
}
