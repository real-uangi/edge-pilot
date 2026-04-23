import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  api,
  getErrorMessage,
  type SchedulerExecutorRecord,
  type SchedulerJobRecord,
  type UpsertSchedulerExecutorInput,
  type UpsertSchedulerJobInput,
} from "../lib/api";
import { ActionButton } from "../components/ActionButton";
import { EmptyState, ErrorState, LoadingState } from "../components/StateBlocks";
import { formatDateTime } from "../lib/format";
import styles from "../styles/admin.module.css";

const scheduleKindLabel: Record<number, string> = {
  1: "一次性",
  2: "Cron",
};

const dispatchPolicyLabel: Record<number, string> = {
  1: "轮询",
  2: "固定 live 槽",
};

const runStatusLabel: Record<number, string> = {
  1: "待执行",
  2: "已派发",
  3: "执行中",
  4: "成功",
  5: "等待重试",
  6: "失败",
};

function newJobForm(): UpsertSchedulerJobInput {
  return {
    name: "",
    taskType: "",
    payload: {},
    scheduleKind: "one_time",
    cronExpr: "*/5 * * * *",
    runAt: new Date().toISOString(),
    dispatchPolicy: "round_robin",
    executorGroup: "default",
    leaseTimeoutSec: 60,
    maxRetries: 3,
    metadata: {},
  };
}

function newExecutorForm(): UpsertSchedulerExecutorInput {
  return {
    executorId: "",
    group: "default",
    enabled: true,
    liveSlot: 0,
    metadata: {},
  };
}

function parseMapJSON(text: string): Record<string, unknown> {
  if (!text.trim()) {
    return {};
  }
  const parsed = JSON.parse(text);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("必须是 JSON 对象");
  }
  return parsed as Record<string, unknown>;
}

export function SchedulerPage() {
  const queryClient = useQueryClient();
  const [jobForm, setJobForm] = useState<UpsertSchedulerJobInput>(() => newJobForm());
  const [executorForm, setExecutorForm] = useState<UpsertSchedulerExecutorInput>(() => newExecutorForm());
  const [jobPayloadText, setJobPayloadText] = useState("{}");
  const [jobMetaText, setJobMetaText] = useState("{}");
  const [executorMetaText, setExecutorMetaText] = useState("{}");
  const [selectedJobId, setSelectedJobId] = useState<string>("");
  const [formError, setFormError] = useState<string>("");

  const jobsQuery = useQuery({ queryKey: ["scheduler", "jobs"], queryFn: api.listSchedulerJobs, refetchInterval: 10000 });
  const executorsQuery = useQuery({ queryKey: ["scheduler", "executors"], queryFn: api.listSchedulerExecutors, refetchInterval: 10000 });
  const selectedJob = useMemo(() => (jobsQuery.data ?? []).find((job) => job.id === selectedJobId), [jobsQuery.data, selectedJobId]);
  const runsQuery = useQuery({
    queryKey: ["scheduler", "runs", selectedJobId],
    queryFn: () => api.listSchedulerRuns(selectedJobId),
    enabled: selectedJobId.length > 0,
    refetchInterval: 5000,
  });

  const refreshAll = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["scheduler", "jobs"] }),
      queryClient.invalidateQueries({ queryKey: ["scheduler", "executors"] }),
      queryClient.invalidateQueries({ queryKey: ["scheduler", "runs"] }),
    ]);
  };

  const createJobMutation = useMutation({
    mutationFn: api.createSchedulerJob,
    onSuccess: async () => {
      setJobForm(newJobForm());
      setJobPayloadText("{}");
      setJobMetaText("{}");
      setFormError("");
      await refreshAll();
    },
  });

  const triggerMutation = useMutation({
    mutationFn: (id: string) => api.triggerSchedulerJob(id),
    onSuccess: refreshAll,
  });

  const toggleJobMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? api.disableSchedulerJob(id) : api.enableSchedulerJob(id),
    onSuccess: refreshAll,
  });

  const deleteJobMutation = useMutation({
    mutationFn: api.deleteSchedulerJob,
    onSuccess: async () => {
      setSelectedJobId("");
      await refreshAll();
    },
  });

  const createExecutorMutation = useMutation({
    mutationFn: api.createSchedulerExecutor,
    onSuccess: async () => {
      setExecutorForm(newExecutorForm());
      setExecutorMetaText("{}");
      await refreshAll();
    },
  });

  const resetExecutorTokenMutation = useMutation({ mutationFn: api.resetSchedulerExecutorToken, onSuccess: refreshAll });
  const toggleExecutorMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? api.disableSchedulerExecutor(id) : api.enableSchedulerExecutor(id),
    onSuccess: refreshAll,
  });
  const deleteExecutorMutation = useMutation({ mutationFn: api.deleteSchedulerExecutor, onSuccess: refreshAll });

  const onCreateJob = () => {
    try {
      setFormError("");
      const payload = parseMapJSON(jobPayloadText);
      const metadata = parseMapJSON(jobMetaText) as Record<string, string>;
      createJobMutation.mutate({ ...jobForm, payload, metadata });
    } catch (error) {
      setFormError(getErrorMessage(error));
    }
  };

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
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>调度中心</h1>
          <p className={styles.sectionCopy}>统一管理定时任务、执行历史与执行器凭证。</p>
        </div>
        <ActionButton label="刷新" onClick={refreshAll} />
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>新建任务</h2>
          </div>
          <ActionButton label="创建任务" variant="primary" pending={createJobMutation.isPending} onClick={onCreateJob} />
        </div>
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>名称</span>
            <input className={styles.input} value={jobForm.name} onChange={(event) => setJobForm((v) => ({ ...v, name: event.target.value }))} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>taskType</span>
            <input className={styles.input} value={jobForm.taskType} onChange={(event) => setJobForm((v) => ({ ...v, taskType: event.target.value }))} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>scheduleKind</span>
            <select className={styles.select} value={jobForm.scheduleKind} onChange={(event) => setJobForm((v) => ({ ...v, scheduleKind: event.target.value as "one_time" | "cron" }))}>
              <option value="one_time">one_time</option>
              <option value="cron">cron</option>
            </select>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>dispatchPolicy</span>
            <select className={styles.select} value={jobForm.dispatchPolicy ?? "round_robin"} onChange={(event) => setJobForm((v) => ({ ...v, dispatchPolicy: event.target.value as "round_robin" | "fixed_live_slot" }))}>
              <option value="round_robin">round_robin</option>
              <option value="fixed_live_slot">fixed_live_slot</option>
            </select>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>executorGroup</span>
            <input className={styles.input} value={jobForm.executorGroup} onChange={(event) => setJobForm((v) => ({ ...v, executorGroup: event.target.value }))} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>leaseTimeoutSec</span>
            <input className={styles.input} type="number" value={jobForm.leaseTimeoutSec ?? 60} onChange={(event) => setJobForm((v) => ({ ...v, leaseTimeoutSec: Number(event.target.value) || 60 }))} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>maxRetries</span>
            <input className={styles.input} type="number" value={jobForm.maxRetries ?? 3} onChange={(event) => setJobForm((v) => ({ ...v, maxRetries: Number(event.target.value) || 0 }))} />
          </label>
          {jobForm.scheduleKind === "cron" ? (
            <label className={styles.field}>
              <span className={styles.label}>cronExpr</span>
              <input className={styles.input} value={jobForm.cronExpr ?? ""} onChange={(event) => setJobForm((v) => ({ ...v, cronExpr: event.target.value }))} />
            </label>
          ) : (
            <label className={styles.field}>
              <span className={styles.label}>runAt (UTC ISO)</span>
              <input className={styles.input} value={jobForm.runAt ?? ""} onChange={(event) => setJobForm((v) => ({ ...v, runAt: event.target.value }))} />
            </label>
          )}
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>payload(JSON)</span>
            <textarea className={styles.textarea} rows={4} value={jobPayloadText} onChange={(event) => setJobPayloadText(event.target.value)} />
          </label>
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>metadata(JSON)</span>
            <textarea className={styles.textarea} rows={2} value={jobMetaText} onChange={(event) => setJobMetaText(event.target.value)} />
          </label>
        </div>
        {formError ? <div className={styles.inlineError}>{formError}</div> : null}
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>任务定义</h2>
          </div>
        </div>
        {jobsQuery.isPending ? (
          <LoadingState title="正在加载任务定义" message="正在读取调度任务列表。" />
        ) : jobsQuery.isError ? (
          <ErrorState title="任务列表加载失败" message={getErrorMessage(jobsQuery.error)} onRetry={() => jobsQuery.refetch()} />
        ) : !(jobsQuery.data ?? []).length ? (
          <EmptyState title="暂无任务" message="先创建一个调度任务。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>名称</th>
                  <th>类型</th>
                  <th>计划</th>
                  <th>分发</th>
                  <th>组</th>
                  <th>下次执行</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(jobsQuery.data ?? []).map((job: SchedulerJobRecord) => (
                  <tr key={job.id} className={job.id === selectedJobId ? styles.tableRowSelected : ""}>
                    <td>
                      <button className={styles.ghostButton} onClick={() => setSelectedJobId(job.id)} type="button">
                        {job.name}
                      </button>
                    </td>
                    <td>{job.taskType}</td>
                    <td>{scheduleKindLabel[job.scheduleKind] ?? String(job.scheduleKind)}</td>
                    <td>{dispatchPolicyLabel[job.dispatchPolicy] ?? String(job.dispatchPolicy)}</td>
                    <td>{job.executorGroup}</td>
                    <td>{job.nextRunAt ? formatDateTime(job.nextRunAt) : "-"}</td>
                    <td>{job.enabled ? "启用" : "停用"}</td>
                    <td className={styles.buttonRow}>
                      <ActionButton label="触发" pending={triggerMutation.isPending} onClick={() => triggerMutation.mutate(job.id)} />
                      <ActionButton
                        label={job.enabled ? "停用" : "启用"}
                        pending={toggleJobMutation.isPending}
                        onClick={() => toggleJobMutation.mutate({ id: job.id, enabled: Boolean(job.enabled) })}
                      />
                      <ActionButton label="删除" variant="danger" pending={deleteJobMutation.isPending} onClick={() => deleteJobMutation.mutate(job.id)} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>执行历史</h2>
            <p className={styles.sectionCopy}>{selectedJob ? `任务：${selectedJob.name}` : "选择任务查看执行记录"}</p>
          </div>
        </div>
        {!selectedJobId ? (
          <EmptyState title="未选择任务" message="点击任务名称查看运行记录。" />
        ) : runsQuery.isPending ? (
          <LoadingState title="正在加载执行记录" message="读取最近运行结果。" />
        ) : runsQuery.isError ? (
          <ErrorState title="执行记录加载失败" message={getErrorMessage(runsQuery.error)} onRetry={() => runsQuery.refetch()} />
        ) : !(runsQuery.data ?? []).length ? (
          <EmptyState title="暂无记录" message="该任务还未产生执行记录。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>Run</th>
                  <th>状态</th>
                  <th>Attempt</th>
                  <th>租约执行器</th>
                  <th>调度时间</th>
                  <th>完成时间</th>
                  <th>错误</th>
                </tr>
              </thead>
              <tbody>
                {(runsQuery.data ?? []).map((run) => (
                  <tr key={run.id}>
                    <td>{run.id.slice(0, 8)}</td>
                    <td>{runStatusLabel[run.status] ?? String(run.status)}</td>
                    <td>{run.attempt}/{run.maxRetries}</td>
                    <td>{run.leasedBy || "-"}</td>
                    <td>{formatDateTime(run.scheduledAt)}</td>
                    <td>{run.completedAt ? formatDateTime(run.completedAt) : "-"}</td>
                    <td>{run.errorMessage || "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>执行器凭证</h2>
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
          <label className={styles.field}>
            <span className={styles.label}>liveSlot (0/1/2)</span>
            <input className={styles.input} type="number" value={executorForm.liveSlot ?? 0} onChange={(event) => setExecutorForm((v) => ({ ...v, liveSlot: Number(event.target.value) || 0 }))} />
          </label>
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>metadata(JSON)</span>
            <textarea className={styles.textarea} rows={2} value={executorMetaText} onChange={(event) => setExecutorMetaText(event.target.value)} />
          </label>
        </div>

        {executorsQuery.isPending ? (
          <LoadingState title="正在加载执行器" message="正在读取执行器列表。" />
        ) : executorsQuery.isError ? (
          <ErrorState title="执行器加载失败" message={getErrorMessage(executorsQuery.error)} onRetry={() => executorsQuery.refetch()} />
        ) : !(executorsQuery.data ?? []).length ? (
          <EmptyState title="暂无执行器" message="创建执行器后可通过 SDK 接入。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>组</th>
                  <th>liveSlot</th>
                  <th>状态</th>
                  <th>最近心跳</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {(executorsQuery.data ?? []).map((executor: SchedulerExecutorRecord) => (
                  <tr key={executor.id}>
                    <td>{executor.id}</td>
                    <td>{executor.group}</td>
                    <td>{executor.liveSlot}</td>
                    <td>{executor.enabled ? "启用" : "停用"}</td>
                    <td>{executor.lastSeenAt ? formatDateTime(executor.lastSeenAt) : "-"}</td>
                    <td className={styles.buttonRow}>
                      <ActionButton label="重置Token" pending={resetExecutorTokenMutation.isPending} onClick={() => resetExecutorTokenMutation.mutate(executor.id)} />
                      <ActionButton
                        label={executor.enabled ? "停用" : "启用"}
                        pending={toggleExecutorMutation.isPending}
                        onClick={() => toggleExecutorMutation.mutate({ id: executor.id, enabled: Boolean(executor.enabled) })}
                      />
                      <ActionButton label="删除" variant="danger" pending={deleteExecutorMutation.isPending} onClick={() => deleteExecutorMutation.mutate(executor.id)} />
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
