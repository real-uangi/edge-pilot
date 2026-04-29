import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { schedulerApi } from "../api";
import type { SchedulerJobRecord } from "../types";
import { JobForm } from "./JobForm";
import { scheduleKindLabel, dispatchPolicyLabel } from "./scheduler-utils";
import styles from "../../../styles/admin.module.css";

export function JobsPage() {
  const queryClient = useQueryClient();

  const jobsQuery = useQuery({
    queryKey: ["scheduler", "jobs"],
    queryFn: schedulerApi.listJobs,
    refetchInterval: 10000,
  });

  const triggerMutation = useMutation({
    mutationFn: schedulerApi.triggerJob,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["scheduler"] }),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? schedulerApi.disableJob(id) : schedulerApi.enableJob(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["scheduler"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: schedulerApi.deleteJob,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["scheduler"] }),
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>定时任务</h1>
          <p className={styles.sectionCopy}>管理调度任务定义与触发策略。</p>
        </div>
        <ActionButton label="刷新" onClick={() => jobsQuery.refetch()} />
      </section>

      <JobForm />

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>任务列表</h2>
          </div>
        </div>
        {jobsQuery.isPending ? (
          <LoadingState title="正在加载任务列表" message="正在读取调度任务。" />
        ) : jobsQuery.isError ? (
          <ErrorState title="任务列表加载失败" message={getErrorMessage(jobsQuery.error)} onRetry={() => jobsQuery.refetch()} />
        ) : !jobsQuery.data?.length ? (
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
                {jobsQuery.data.map((job) => (
                  <tr key={job.id}>
                    <td>
                      <Link className={styles.tableLink} to={`/scheduler/${job.id}`}>
                        {job.name}
                      </Link>
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
                        pending={toggleMutation.isPending}
                        onClick={() => toggleMutation.mutate({ id: job.id, enabled: Boolean(job.enabled) })}
                      />
                      <ActionButton label="删除" variant="danger" pending={deleteMutation.isPending} onClick={() => deleteMutation.mutate(job.id)} />
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
