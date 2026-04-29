import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  api,
  getErrorMessage,
  type SchedulerExecutorRecord,
  type UpsertSchedulerExecutorInput,
} from "../../lib/api";
import { ActionButton } from "../ActionButton";
import { EmptyState, ErrorState, LoadingState } from "../StateBlocks";
import { formatDateTime } from "../../lib/format";
import styles from "../../styles/admin.module.css";
import { newExecutorForm, parseMapJSON } from "./scheduler-utils";

interface ExecutorsTabProps {
  executors: SchedulerExecutorRecord[] | undefined;
  executorsLoading: boolean;
  executorsError: Error | null;
  onRefetchExecutors: () => void;
  onRefreshAll: () => Promise<void>;
  resetTokenMutate: (id: string) => void;
  resetTokenPending: boolean;
  toggleMutate: (args: { id: string; enabled: boolean }) => void;
  togglePending: boolean;
  deleteMutate: (id: string) => void;
  deletePending: boolean;
}

export function ExecutorsTab({
  executors,
  executorsLoading,
  executorsError,
  onRefetchExecutors,
  onRefreshAll,
  resetTokenMutate,
  resetTokenPending,
  toggleMutate,
  togglePending,
  deleteMutate,
  deletePending,
}: ExecutorsTabProps) {
  const [executorForm, setExecutorForm] = useState<UpsertSchedulerExecutorInput>(() => newExecutorForm());
  const [executorMetaText, setExecutorMetaText] = useState("{}");
  const [formError, setFormError] = useState("");

  const createExecutorMutation = useMutation({
    mutationFn: api.createSchedulerExecutor,
    onSuccess: async () => {
      setExecutorForm(newExecutorForm());
      setExecutorMetaText("{}");
      setFormError("");
      await onRefreshAll();
    },
  });

  const onCreateExecutor = () => {
    try {
      setFormError("");
      const metadata = parseMapJSON(executorMetaText) as Record<string, string>;
      createExecutorMutation.mutate({ ...executorForm, metadata });
    } catch (error) {
      setFormError(getErrorMessage(error));
    }
  };

  return (
    <>
      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>新建执行器</h2>
          </div>
          <ActionButton label="创建执行器" variant="primary" pending={createExecutorMutation.isPending} onClick={onCreateExecutor} />
        </div>
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>executorId</span>
            <input className={styles.input} value={executorForm.executorId} onChange={(event) => setExecutorForm((v) => ({ ...v, executorId: event.target.value }))} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>group</span>
            <input className={styles.input} value={executorForm.group} onChange={(event) => setExecutorForm((v) => ({ ...v, group: event.target.value }))} />
          </label>
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>metadata(JSON)</span>
            <textarea className={styles.textarea} rows={2} value={executorMetaText} onChange={(event) => setExecutorMetaText(event.target.value)} />
          </label>
        </div>
        {formError ? <div className={styles.inlineError}>{formError}</div> : null}
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>执行器列表</h2>
          </div>
        </div>
        {executorsLoading ? (
          <LoadingState title="正在加载执行器" message="正在读取执行器列表。" />
        ) : executorsError ? (
          <ErrorState title="执行器加载失败" message={getErrorMessage(executorsError)} onRetry={onRefetchExecutors} />
        ) : !(executors ?? []).length ? (
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
                {(executors ?? []).map((executor: SchedulerExecutorRecord) => (
                  <tr key={executor.id}>
                    <td>{executor.id}</td>
                    <td>{executor.group}</td>
                    <td>{executor.channelMode === 2 ? "agent_relay" : "direct"}</td>
                    <td>{executor.liveSlot}</td>
                    <td>{executor.enabled ? "启用" : "停用"}</td>
                    <td>{executor.lastSeenAt ? formatDateTime(executor.lastSeenAt) : "-"}</td>
                    <td className={styles.buttonRow}>
                      <ActionButton label="重置Token" pending={resetTokenPending} onClick={() => resetTokenMutate(executor.id)} />
                      <ActionButton
                        label={executor.enabled ? "停用" : "启用"}
                        pending={togglePending}
                        onClick={() => toggleMutate({ id: executor.id, enabled: Boolean(executor.enabled) })}
                      />
                      <ActionButton label="删除" variant="danger" pending={deletePending} onClick={() => deleteMutate(executor.id)} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  );
}
