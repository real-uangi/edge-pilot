import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { useDialog } from "../../../shared/components/DialogProvider";
import { schedulerApi } from "../api";
import type { SchedulerJobRecord } from "../types";
import { JobForm } from "./JobForm";
import { scheduleKindLabel, dispatchPolicyLabel } from "./scheduler-utils";
import styles from "../../../styles/admin.module.css";

export function JobsPage() {
  const queryClient = useQueryClient();
  const dialog = useDialog();

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

  const handleTrigger = async (job: SchedulerJobRecord) => {
    const confirmed = await dialog.confirm({
      title: "立即触发任务",
      message: `确认立即触发任务 "${job.name}"？`,
      confirmText: "确认触发",
      cancelText: "取消",
    });
    if (confirmed) {
      triggerMutation.mutate(job.id);
    }
  };

  const handleDelete = async (job: SchedulerJobRecord) => {
    const confirmed = await dialog.confirm({
      title: "删除任务",
      message: `确认删除任务 "${job.name}"？删除后无法恢复。`,
      confirmText: "确认删除",
      cancelText: "取消",
      danger: true,
    });
    if (confirmed) {
      deleteMutation.mutate(job.id);
    }
  };

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
                  <th>Handler</th>
                  <th>关联服务</th>
                  <th>调度方式</th>
                  <th>分发策略</th>
                  <th>执行器组</th>
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
                    <td>{job.handlerKey}</td>
                    <td>{job.serviceId ? job.serviceId.slice(0, 8) : "-"}</td>
                    <td>{scheduleKindLabel[job.scheduleKind] ?? String(job.scheduleKind)}</td>
                    <td>{dispatchPolicyLabel[job.dispatchPolicy] ?? String(job.dispatchPolicy)}</td>
                    <td>{job.executorGroup}</td>
                    <td>{job.nextRunAt ? formatDateTime(job.nextRunAt) : "-"}</td>
                    <td>{job.enabled ? "启用" : "停用"}</td>
                    <td className={styles.buttonRow}>
                      <ActionButton label="触发" pending={triggerMutation.isPending} onClick={() => handleTrigger(job)} />
                      <ActionButton
                        label={job.enabled ? "停用" : "启用"}
                        pending={toggleMutation.isPending}
                        onClick={() => toggleMutation.mutate({ id: job.id, enabled: Boolean(job.enabled) })}
                      />
                      <ActionButton label="删除" variant="danger" pending={deleteMutation.isPending} onClick={() => handleDelete(job)} />
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
