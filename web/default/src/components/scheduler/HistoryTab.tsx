import { getErrorMessage, type SchedulerRunRecord } from "../../lib/api";
import { EmptyState, ErrorState, LoadingState } from "../StateBlocks";
import { formatDateTime } from "../../lib/format";
import styles from "../../styles/admin.module.css";
import { runStatusLabel } from "./scheduler-utils";

interface HistoryTabProps {
  selectedJobId: string;
  selectedJobName: string | undefined;
  runs: SchedulerRunRecord[] | undefined;
  runsLoading: boolean;
  runsError: Error | null;
  onRefetchRuns: () => void;
}

export function HistoryTab({
  selectedJobId,
  selectedJobName,
  runs,
  runsLoading,
  runsError,
  onRefetchRuns,
}: HistoryTabProps) {
  if (!selectedJobId) {
    return (
      <section className={styles.sectionCard}>
        <EmptyState
          title="未选择任务"
          message="请先在任务定义中选择一个任务。"
        />
      </section>
    );
  }

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>执行历史</h2>
          <p className={styles.sectionCopy}>{selectedJobName ? `任务：${selectedJobName}` : ""}</p>
        </div>
      </div>
      {runsLoading ? (
        <LoadingState title="正在加载执行记录" message="读取最近运行结果。" />
      ) : runsError ? (
        <ErrorState title="执行记录加载失败" message={getErrorMessage(runsError)} onRetry={onRefetchRuns} />
      ) : !(runs ?? []).length ? (
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
              {(runs ?? []).map((run) => (
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
  );
}
