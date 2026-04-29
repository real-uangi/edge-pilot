import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  api,
  getErrorMessage,
  type SchedulerJobRecord,
  type UpsertSchedulerJobInput,
} from "../../lib/api";
import { ActionButton } from "../ActionButton";
import { EmptyState, ErrorState, LoadingState } from "../StateBlocks";
import { formatDateTime } from "../../lib/format";
import styles from "../../styles/admin.module.css";
import { scheduleKindLabel, dispatchPolicyLabel, newJobForm, parseMapJSON } from "./scheduler-utils";

interface JobsTabProps {
  jobs: SchedulerJobRecord[];
  jobsLoading: boolean;
  jobsError: Error | null;
  onRefetchJobs: () => void;
  selectedJobId: string;
  onSelectJob: (id: string) => void;
  onRefreshAll: () => Promise<void>;
  triggerMutate: (id: string) => void;
  triggerPending: boolean;
  toggleMutate: (args: { id: string; enabled: boolean }) => void;
  togglePending: boolean;
  deleteMutate: (id: string) => void;
  deletePending: boolean;
}

export function JobsTab({
  jobs,
  jobsLoading,
  jobsError,
  onRefetchJobs,
  selectedJobId,
  onSelectJob,
  onRefreshAll,
  triggerMutate,
  triggerPending,
  toggleMutate,
  togglePending,
  deleteMutate,
  deletePending,
}: JobsTabProps) {
  const [jobForm, setJobForm] = useState<UpsertSchedulerJobInput>(() => newJobForm());
  const [jobPayloadText, setJobPayloadText] = useState("{}");
  const [jobMetaText, setJobMetaText] = useState("{}");
  const [formError, setFormError] = useState("");

  const createJobMutation = useMutation({
    mutationFn: api.createSchedulerJob,
    onSuccess: async () => {
      setJobForm(newJobForm());
      setJobPayloadText("{}");
      setJobMetaText("{}");
      setFormError("");
      await onRefreshAll();
    },
  });

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

  return (
    <>
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
        {jobsLoading ? (
          <LoadingState title="正在加载任务定义" message="正在读取调度任务列表。" />
        ) : jobsError ? (
          <ErrorState title="任务列表加载失败" message={getErrorMessage(jobsError)} onRetry={onRefetchJobs} />
        ) : !jobs.length ? (
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
                {jobs.map((job: SchedulerJobRecord) => (
                  <tr key={job.id} className={job.id === selectedJobId ? styles.tableRowSelected : ""}>
                    <td>
                      <button className={styles.ghostButton} onClick={() => onSelectJob(job.id)} type="button">
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
                      <ActionButton label="触发" pending={triggerPending} onClick={() => triggerMutate(job.id)} />
                      <ActionButton
                        label={job.enabled ? "停用" : "启用"}
                        pending={togglePending}
                        onClick={() => toggleMutate({ id: job.id, enabled: Boolean(job.enabled) })}
                      />
                      <ActionButton label="删除" variant="danger" pending={deletePending} onClick={() => deleteMutate(job.id)} />
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
