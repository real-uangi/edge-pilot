import { useState } from "react";
import {
  useMutation,
  useIsMutating,
  useMutationState,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime, boolLabel, shortId } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
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
import { servicesApi } from "../../services/api";
import { schedulerApi } from "../api";
import type { SchedulerJobRecord } from "../types";
import { JobForm } from "./JobForm";
import { scheduleKindLabel, dispatchPolicyLabel } from "./scheduler-utils";
import styles from "../../../styles/admin.module.css";

type JobAction = {
  job: SchedulerJobRecord;
  action: "trigger" | "toggle" | "delete";
};

export function JobsPage() {
  const queryClient = useQueryClient();
  const dialog = useDialog();
  const [creating, setCreating] = useState(false);
  const createPending =
    useIsMutating({ mutationKey: ["scheduler", "create-job"] }) > 0;
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [feedback, setFeedback] = useState<{
    message: string;
    error: boolean;
  } | null>(null);
  const jobsQuery = useQuery({
    queryKey: ["scheduler", "jobs"],
    queryFn: schedulerApi.listJobs,
    refetchInterval: 10000,
  });
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: servicesApi.list,
  });
  const serviceNames = new Map(
    servicesQuery.data?.map((service) => [service.id, service.name]),
  );
  const actionMutation = useMutation({
    mutationKey: ["scheduler", "job-action"],
    mutationFn: async ({ job, action }: JobAction) => {
      if (action === "trigger") await schedulerApi.triggerJob(job.id);
      else if (action === "delete") await schedulerApi.deleteJob(job.id);
      else if (job.enabled) await schedulerApi.disableJob(job.id);
      else await schedulerApi.enableJob(job.id);
    },
    onMutate: () => setFeedback(null),
    onSuccess: async (_, { job, action }) => {
      setFeedback({
        message:
          action === "trigger"
            ? `已提交「${job.name}」的执行请求`
            : action === "delete"
              ? `已删除「${job.name}」`
              : `已${job.enabled ? "停用" : "启用"}「${job.name}」`,
        error: false,
      });
      await queryClient.invalidateQueries({ queryKey: ["scheduler"] });
    },
    onError: (error, { job }) =>
      setFeedback({
        message: `${job.name}：${getErrorMessage(error)}`,
        error: true,
      }),
  });
  const pending = useMutationState({
    filters: { mutationKey: ["scheduler", "job-action"], status: "pending" },
    select: (mutation) => mutation.state.variables as JobAction,
  });
  const handleAction = async (
    job: SchedulerJobRecord,
    action: JobAction["action"],
  ) => {
    if (
      action !== "toggle" &&
      !(await dialog.confirm({
        title: action === "delete" ? "删除任务" : "立即触发任务",
        message:
          action === "delete"
            ? `确认删除任务「${job.name}」及其执行记录？此操作无法撤销。`
            : `确认立即触发任务「${job.name}」？`,
        confirmText: action === "delete" ? "删除任务" : "立即触发",
        danger: action === "delete",
      }))
    )
      return;
    actionMutation.mutate({ job, action });
  };
  const query = search.trim().toLowerCase();
  const jobs = (jobsQuery.data ?? []).filter(
    (job) =>
      (!query ||
        [
          job.name,
          job.handlerKey,
          job.executorGroup,
          job.serviceId
            ? (serviceNames.get(job.serviceId) ?? job.serviceId)
            : "",
        ].some((value) => value.toLowerCase().includes(query))) &&
      (status === "all" ||
        (status === "enabled" ? job.enabled === true : job.enabled === false)),
  );

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <h1 className={styles.sectionTitle}>定时任务</h1>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新"
            pending={jobsQuery.isFetching}
            onClick={() => void jobsQuery.refetch()}
          />
          <ActionButton
            label="新建任务"
            disabled={createPending}
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
      <section className={styles.sectionCard}>
        <div className={styles.listToolbar}>
          <label className={styles.searchField}>
            <span className={styles.visuallyHidden}>搜索任务</span>
            <input
              className={styles.input}
              type="search"
              placeholder="搜索任务、Handler 或关联服务"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <label className={styles.filterGroup}>
            <span className={styles.visuallyHidden}>任务状态</span>
            <select
              className={styles.select}
              value={status}
              onChange={(event) => setStatus(event.target.value)}
            >
              <option value="all">全部状态</option>
              <option value="enabled">启用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <span className={styles.resultCount} aria-live="polite">
            {jobs.length} 个任务
          </span>
        </div>
        <QueryRefreshNotice
          query={jobsQuery}
          onRetry={() => void jobsQuery.refetch()}
        />
        {jobsQuery.isPending ? (
          <LoadingState title="正在加载任务" />
        ) : jobsQuery.isError && !jobsQuery.data ? (
          <ErrorState
            title="任务加载失败"
            message={getErrorMessage(jobsQuery.error)}
            onRetry={() => void jobsQuery.refetch()}
          />
        ) : !jobsQuery.data?.length ? (
          <EmptyState
            title="暂无任务"
            actionLabel="新建任务"
            onRetry={() => setCreating(true)}
          />
        ) : !jobs.length ? (
          <EmptyState
            title="没有匹配的任务"
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
                  <th>任务 / Handler</th>
                  <th>关联服务</th>
                  <th>调度</th>
                  <th>下次执行</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {jobs.map((job) => {
                  const pendingAction = pending.find(
                    (item) => item.job.id === job.id,
                  )?.action;
                  return (
                    <tr key={job.id}>
                      <td data-label="任务">
                        <Link
                          className={styles.tableLink}
                          to={`/scheduler/history?jobId=${encodeURIComponent(job.id)}`}
                        >
                          {job.name}
                        </Link>
                        <div className={styles.muted}>
                          <code>{job.handlerKey}</code>
                        </div>
                      </td>
                      <td data-label="服务">
                        {job.serviceId ? (
                          <Link
                            className={styles.tableLink}
                            to={`/services/${job.serviceId}`}
                          >
                            {serviceNames.get(job.serviceId) ??
                              shortId(job.serviceId)}
                          </Link>
                        ) : (
                          "—"
                        )}
                        <div className={styles.muted}>
                          <code>{job.executorGroup}</code>
                        </div>
                      </td>
                      <td data-label="调度">
                        {scheduleKindLabel[job.scheduleKind] ??
                          String(job.scheduleKind)}
                        {job.scheduleKind === 2 ? (
                          <div>
                            <code>{job.cronExpr}</code>
                          </div>
                        ) : null}
                        <div className={styles.muted}>
                          {dispatchPolicyLabel[job.dispatchPolicy] ??
                            String(job.dispatchPolicy)}
                        </div>
                      </td>
                      <td data-label="下次执行">
                        {formatDateTime(job.nextRunAt)}
                      </td>
                      <td data-label="状态">
                        <StatusPill
                          label={boolLabel(job.enabled, "启用", "停用")}
                          tone={job.enabled ? "success" : "default"}
                        />
                      </td>
                      <td data-label="操作">
                        <div className={styles.buttonRow}>
                          <ActionButton
                            label="立即触发"
                            pending={pendingAction === "trigger"}
                            disabled={Boolean(pendingAction)}
                            onClick={() => void handleAction(job, "trigger")}
                          />
                          <ActionButton
                            label={job.enabled ? "停用" : "启用"}
                            pending={pendingAction === "toggle"}
                            disabled={Boolean(pendingAction)}
                            variant="ghost"
                            onClick={() => void handleAction(job, "toggle")}
                          />
                          <ActionButton
                            label="删除"
                            pending={pendingAction === "delete"}
                            disabled={Boolean(pendingAction)}
                            variant="danger"
                            onClick={() => void handleAction(job, "delete")}
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
          title="新建任务"
          variant="wide"
          closeDisabled={createPending}
          onClose={() => setCreating(false)}
        >
          <JobForm
            onCreated={() => {
              setCreating(false);
              setFeedback({ message: "任务已创建", error: false });
            }}
            onCancel={() => setCreating(false)}
          />
        </Modal>
      ) : null}
    </div>
  );
}
