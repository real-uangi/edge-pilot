import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime, shortId } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { StatusPill } from "../../../shared/components/StatusPill";
import { schedulerApi } from "../api";
import { runStatusLabel } from "./scheduler-utils";
import styles from "../../../styles/admin.module.css";

export function RunsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedJobId = searchParams.get("jobId") ?? "";
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const jobsQuery = useQuery({
    queryKey: ["scheduler", "jobs"],
    queryFn: schedulerApi.listJobs,
  });
  const runsQuery = useQuery({
    queryKey: ["scheduler", "runs", selectedJobId || "all"],
    queryFn: () =>
      selectedJobId
        ? schedulerApi.listRuns(selectedJobId)
        : schedulerApi.listAllRuns(),
    refetchInterval: 5000,
  });
  const jobNames = new Map(jobsQuery.data?.map((job) => [job.id, job.name]));
  const query = search.trim().toLowerCase();
  const runs = (runsQuery.data ?? []).filter(
    (run) =>
      (status === "all" || String(run.status) === status) &&
      (!query ||
        [
          run.id,
          run.handlerKey,
          run.leasedBy,
          run.errorMessage,
          jobNames.get(run.jobId) ?? run.jobId,
        ].some((value) => value.toLowerCase().includes(query))),
  );
  const selectJob = (id: string) =>
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      if (id) next.set("jobId", id);
      else next.delete("jobId");
      return next;
    });

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div className={styles.pageTitleGroup}>
          <h1 className={styles.sectionTitle}>执行历史</h1>
          <span className={styles.refreshMeta}>最近 50 条记录</span>
        </div>
        <div className={styles.buttonRow}>
          <Link className={styles.ghostButton} to="/scheduler">
            返回任务
          </Link>
          <ActionButton
            label="刷新"
            pending={runsQuery.isFetching}
            onClick={() => void runsQuery.refetch()}
          />
        </div>
      </header>
      <section className={styles.sectionCard}>
        <div className={styles.listToolbar}>
          <label className={styles.searchField}>
            <span className={styles.visuallyHidden}>搜索执行记录</span>
            <input
              className={styles.input}
              type="search"
              placeholder="搜索任务、Handler、执行器或错误"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <label className={styles.filterGroup}>
            <span className={styles.visuallyHidden}>按任务筛选</span>
            <select
              className={styles.select}
              value={selectedJobId}
              onChange={(event) => selectJob(event.target.value)}
            >
              <option value="">全部任务</option>
              {selectedJobId && !jobNames.has(selectedJobId) ? (
                <option value={selectedJobId}>
                  所选任务 · {shortId(selectedJobId)}
                </option>
              ) : null}
              {jobsQuery.data?.map((job) => (
                <option key={job.id} value={job.id}>
                  {job.name}
                </option>
              ))}
            </select>
          </label>
          <label className={styles.filterGroup}>
            <span className={styles.visuallyHidden}>执行状态</span>
            <select
              className={styles.select}
              value={status}
              onChange={(event) => setStatus(event.target.value)}
            >
              <option value="all">全部状态</option>
              {Object.entries(runStatusLabel).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>
          <span className={styles.resultCount} aria-live="polite">
            {runs.length} 条记录
          </span>
        </div>
        {jobsQuery.isError ? (
          <div className={styles.inlineRow}>
            <InlineNotice message="任务列表加载失败，暂时使用任务 ID 显示。" />
            <ActionButton
              label="重试任务列表"
              pending={jobsQuery.isFetching}
              onClick={() => void jobsQuery.refetch()}
            />
          </div>
        ) : null}
        <QueryRefreshNotice
          query={runsQuery}
          onRetry={() => void runsQuery.refetch()}
        />
        {runsQuery.isPending ? (
          <LoadingState title="正在加载执行记录" />
        ) : runsQuery.isError && !runsQuery.data ? (
          <ErrorState
            title="执行记录加载失败"
            message={getErrorMessage(runsQuery.error)}
            onRetry={() => void runsQuery.refetch()}
          />
        ) : !runsQuery.data?.length ? (
          <EmptyState
            title="暂无执行记录"
            message={selectedJobId ? "该任务还未产生执行记录。" : undefined}
          />
        ) : !runs.length ? (
          <EmptyState
            title="没有匹配的执行记录"
            actionLabel="清除搜索与状态筛选"
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
                  <th>任务 / Handler</th>
                  <th>执行编号</th>
                  <th>状态</th>
                  <th>尝试次数</th>
                  <th>执行器</th>
                  <th>执行时间</th>
                  <th>错误信息</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run) => (
                  <tr key={run.id}>
                    <td data-label="任务">
                      <Link
                        className={styles.tableLink}
                        to={`/scheduler/history?jobId=${encodeURIComponent(run.jobId)}`}
                      >
                        {jobNames.get(run.jobId) ?? shortId(run.jobId)}
                      </Link>
                      <div className={styles.muted}>
                        <code>{run.handlerKey}</code>
                      </div>
                    </td>
                    <td data-label="执行编号">
                      <code title={run.id}>{shortId(run.id)}</code>
                    </td>
                    <td data-label="状态">
                      <StatusPill
                        label={runStatusLabel[run.status] ?? String(run.status)}
                        tone={
                          run.status === 4
                            ? "success"
                            : run.status === 6
                              ? "danger"
                              : run.status === 5
                                ? "warning"
                                : "default"
                        }
                      />
                    </td>
                    <td data-label="尝试次数">
                      第 {run.attempt} 次
                      <div className={styles.muted}>
                        重试上限 {run.maxRetries}
                      </div>
                    </td>
                    <td data-label="执行器">
                      <code>{run.leasedBy || "—"}</code>
                    </td>
                    <td data-label="执行时间">
                      <div>{formatDateTime(run.scheduledAt)}</div>
                      {run.completedAt ? (
                        <div className={styles.muted}>
                          完成 {formatDateTime(run.completedAt)}
                        </div>
                      ) : null}
                    </td>
                    <td data-label="错误信息">{run.errorMessage || "—"}</td>
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
