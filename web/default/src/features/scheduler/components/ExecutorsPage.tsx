import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../../../shared/components/StateBlocks";
import { schedulerApi } from "../api";
import { ExecutorForm } from "./ExecutorForm";
import styles from "../../../styles/admin.module.css";

export function ExecutorsPage() {
  const queryClient = useQueryClient();

  const executorsQuery = useQuery({
    queryKey: ["scheduler", "executors"],
    queryFn: schedulerApi.listExecutors,
    refetchInterval: 10000,
  });

  const resetTokenMutation = useMutation({
    mutationFn: schedulerApi.resetExecutorToken,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["scheduler", "executors"] }),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? schedulerApi.disableExecutor(id) : schedulerApi.enableExecutor(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["scheduler", "executors"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: schedulerApi.deleteExecutor,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["scheduler", "executors"] }),
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>执行器</h1>
          <p className={styles.sectionCopy}>管理调度执行器凭证与状态。</p>
        </div>
        <ActionButton label="刷新" onClick={() => executorsQuery.refetch()} />
      </section>

      <ExecutorForm />

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>执行器列表</h2>
          </div>
        </div>
        {executorsQuery.isPending ? (
          <LoadingState title="正在加载执行器" message="正在读取执行器列表。" />
        ) : executorsQuery.isError ? (
          <ErrorState title="执行器加载失败" message={getErrorMessage(executorsQuery.error)} onRetry={() => executorsQuery.refetch()} />
        ) : !executorsQuery.data?.length ? (
          <EmptyState title="暂无执行器" message="创建执行器后可通过 SDK 接入。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>组</th>
                  <th>通道</th>
                  <th>liveSlot</th>
                  <th>状态</th>
                  <th>最近心跳</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {executorsQuery.data.map((executor) => (
                  <tr key={executor.id}>
                    <td>{executor.id}</td>
                    <td>{executor.group}</td>
                    <td>{executor.channelMode === 2 ? "agent_relay" : "direct"}</td>
                    <td>{executor.liveSlot}</td>
                    <td>{executor.enabled ? "启用" : "停用"}</td>
                    <td>{executor.lastSeenAt ? formatDateTime(executor.lastSeenAt) : "-"}</td>
                    <td className={styles.buttonRow}>
                      <ActionButton label="重置Token" pending={resetTokenMutation.isPending} onClick={() => resetTokenMutation.mutate(executor.id)} />
                      <ActionButton
                        label={executor.enabled ? "停用" : "启用"}
                        pending={toggleMutation.isPending}
                        onClick={() => toggleMutation.mutate({ id: executor.id, enabled: Boolean(executor.enabled) })}
                      />
                      <ActionButton label="删除" variant="danger" pending={deleteMutation.isPending} onClick={() => deleteMutation.mutate(executor.id)} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
