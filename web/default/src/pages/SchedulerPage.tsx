import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { api } from "../lib/api";
import { ActionButton } from "../components/ActionButton";
import styles from "../styles/admin.module.css";
import { SchedulerTabs, type SchedulerTab } from "../components/scheduler/SchedulerTabs";
import { JobsTab } from "../components/scheduler/JobsTab";
import { HistoryTab } from "../components/scheduler/HistoryTab";
import { ExecutorsTab } from "../components/scheduler/ExecutorsTab";

export function SchedulerPage() {
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab: SchedulerTab = (searchParams.get("tab") as SchedulerTab) ?? "jobs";
  const [selectedJobId, setSelectedJobId] = useState("");

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

  const resetExecutorTokenMutation = useMutation({ mutationFn: api.resetSchedulerExecutorToken, onSuccess: refreshAll });
  const toggleExecutorMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled ? api.disableSchedulerExecutor(id) : api.enableSchedulerExecutor(id),
    onSuccess: refreshAll,
  });
  const deleteExecutorMutation = useMutation({ mutationFn: api.deleteSchedulerExecutor, onSuccess: refreshAll });

  const handleSelectJob = (id: string) => {
    setSelectedJobId(id);
    setSearchParams({ tab: "history" });
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

      <SchedulerTabs active={activeTab} />
      {activeTab === "jobs" && (
        <JobsTab
          jobs={jobsQuery.data ?? []}
          jobsLoading={jobsQuery.isPending}
          jobsError={jobsQuery.error}
          onRefetchJobs={() => jobsQuery.refetch()}
          selectedJobId={selectedJobId}
          onSelectJob={handleSelectJob}
          onRefreshAll={refreshAll}
          triggerMutate={(id) => triggerMutation.mutate(id)}
          triggerPending={triggerMutation.isPending}
          toggleMutate={(args) => toggleJobMutation.mutate(args)}
          togglePending={toggleJobMutation.isPending}
          deleteMutate={(id) => deleteJobMutation.mutate(id)}
          deletePending={deleteJobMutation.isPending}
        />
      )}
      {activeTab === "history" && (
        <HistoryTab
          selectedJobId={selectedJobId}
          selectedJobName={selectedJob?.name}
          runs={runsQuery.data}
          runsLoading={runsQuery.isPending}
          runsError={runsQuery.error}
          onRefetchRuns={() => runsQuery.refetch()}
        />
      )}
      {activeTab === "executors" && (
        <ExecutorsTab
          executors={executorsQuery.data}
          executorsLoading={executorsQuery.isPending}
          executorsError={executorsQuery.error}
          onRefetchExecutors={() => executorsQuery.refetch()}
          onRefreshAll={refreshAll}
          resetTokenMutate={(id) => resetExecutorTokenMutation.mutate(id)}
          resetTokenPending={resetExecutorTokenMutation.isPending}
          toggleMutate={(args) => toggleExecutorMutation.mutate(args)}
          togglePending={toggleExecutorMutation.isPending}
          deleteMutate={(id) => deleteExecutorMutation.mutate(id)}
          deletePending={deleteExecutorMutation.isPending}
        />
      )}
    </div>
  );
}
