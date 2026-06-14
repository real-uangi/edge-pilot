import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { schedulerApi } from "../api";
import { runStatusLabel } from "./scheduler-utils";
import styles from "../../../styles/admin.module.css";

export function RunsPage() {
  const [selectedJobId, setSelectedJobId] = useState("");

  const jobsQuery = useQuery({
    queryKey: ["scheduler", "jobs"],
    queryFn: schedulerApi.listJobs,
  });

  const runsQuery = useQuery({
    queryKey: ["scheduler", "runs", selectedJobId || "all"],
    queryFn: () =>
      selectedJobId ? schedulerApi.listRuns(selectedJobId) : schedulerApi.listAllRuns(),
    refetchInterval: 5000,
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>执行历史</h1>
          <p className={styles.sectionCopy}>查看调度任务的执行记录与结果。</p>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton label="刷新" onClick={() => runsQuery.refetch()} />
        </div>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>按任务筛选</span>
            <select
              className={styles.select}
              value={selectedJobId}
              onChange={(e) => setSelectedJobId(e.target.value)}
            >
              <option value="">全部任务</option>
              {(jobsQuery.data ?? []).map((job) => (
                <option key={job.id} value={job.id}>
                  {job.name}
                </option>
              ))}
            </select>
          </label>
        </div>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>执行记录</h2>
          </div>
        </div>
        {runsQuery.isPending ? (
          <LoadingState title="正在加载执行记录" message="读取最近运行结果。" />
        ) : runsQuery.isError ? (
          <ErrorState title="执行记录加载失败" message={getErrorMessage(runsQuery.error)} onRetry={() => runsQuery.refetch()} />
        ) : !runsQuery.data?.length ? (
          <EmptyState title="暂无记录" message={selectedJobId ? "该任务还未产生执行记录。" : "暂无执行记录。"} />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>执行编号</th>
                  <th>状态</th>
                  <th>当前重试</th>
                  <th>租约执行器</th>
                  <th>调度时间</th>
                  <th>完成时间</th>
                  <th>错误信息</th>
                </tr>
              </thead>
              <tbody>
                {runsQuery.data.map((run) => (
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
    </div>
  );
}
