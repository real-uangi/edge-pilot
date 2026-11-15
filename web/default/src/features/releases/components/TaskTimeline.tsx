import { useEffect, useState } from "react";
import {
  formatDateTime,
  formatDuration,
  taskStatusLabel,
  taskStatusTone,
} from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState } from "../../../shared/components/StateBlocks";
import { TaskLogModal } from "./TaskLogModal";
import type {
  PipelineCell,
  PipelineStage,
  ReleasePipeline,
  TaskSnapshot,
} from "../types";
import styles from "./PipelineTable.module.css";

interface TaskTimelineProps {
  pipeline: ReleasePipeline;
  tasks: TaskSnapshot[];
}

function durationLabel(duration: number | null): string {
  if (duration === null) return "—";
  return duration > 0 && duration < 1000 ? "<1s" : formatDuration(duration);
}

function stageStatusLabel(cell: PipelineCell, stage: PipelineStage): string {
  if (cell.status === "waiting") {
    return stage.kind === "manual" ? "等待确认" : "等待派发";
  }
  if (cell.status === "completed") {
    if (stage.id === "traffic") return "已切流";
    if (stage.id === "rollback") return "已回滚";
    if (stage.kind === "manual") return "已结束";
    return "完成";
  }
  return {
    pending: "未执行",
    running: "运行中",
    failed: "失败",
    timed_out: "超时",
    unrecorded: "未记录",
    skipped: "未执行",
  }[cell.status];
}

function StageCell({
  cell,
  stage,
  sequence,
  now,
  onInspect,
}: {
  cell: PipelineCell;
  stage: PipelineStage;
  sequence: number;
  now: number;
  onInspect: () => void;
}) {
  const duration = cell.runningSince
    ? (cell.durationMs ?? 0) +
      Math.max(0, now - new Date(cell.runningSince).getTime())
    : cell.durationMs;
  const hasEstimate =
    stage.estimatedDurationMs !== null && stage.estimatedDurationMs > 0;
  const isRunning = stage.kind === "automatic" && cell.status === "running";
  const isFailed = cell.status === "failed" || cell.status === "timed_out";
  const indeterminate = isRunning && !hasEstimate;
  const overEstimate =
    isRunning &&
    hasEstimate &&
    duration !== null &&
    duration > stage.estimatedDurationMs!;
  const progress =
    cell.status === "completed"
      ? 1
      : (isRunning || isFailed) && hasEstimate && duration !== null
        ? Math.min(duration / stage.estimatedDurationMs!, 0.95)
        : 0;
  const status = stageStatusLabel(cell, stage);
  const hasDetails = cell.events.length > 0 || duration !== null || isFailed;
  const showProgress =
    stage.kind === "automatic" &&
    (isRunning || cell.status === "completed" ||
      (isFailed && hasEstimate && duration !== null));

  return (
    <button
      type="button"
      className={styles.cell}
      data-status={cell.status}
      data-error-without-estimate={
        isFailed && (!hasEstimate || duration === null) ? true : undefined
      }
      disabled={!hasDetails}
      onClick={onInspect}
      aria-label={`第 ${sequence} 次执行，${stage.label}，${status}，已用 ${durationLabel(duration)}，查看阶段记录`}
    >
      <span
        className={`${styles.progressFill} ${indeterminate ? styles.indeterminate : ""}`}
        style={indeterminate ? undefined : { transform: `scaleX(${progress})` }}
        role={showProgress ? "progressbar" : undefined}
        aria-hidden={!showProgress}
        aria-label={showProgress ? `${stage.label}预计耗时进度` : undefined}
        aria-valuemin={showProgress ? 0 : undefined}
        aria-valuemax={showProgress ? 100 : undefined}
        aria-valuenow={
          showProgress && !indeterminate ? Math.round(progress * 100) : undefined
        }
        aria-valuetext={
          showProgress
            ? `${status}，已用 ${durationLabel(duration)}，预计 ${durationLabel(stage.estimatedDurationMs)}`
            : undefined
        }
      />
      <span className={styles.cellContent}>
        <span className={styles.duration}>{durationLabel(duration)}</span>
        <span className={styles.cellStatus}>{status}</span>
        {overEstimate ? (
          <span className={styles.overEstimate}>超出预计</span>
        ) : null}
      </span>
    </button>
  );
}

export function TaskTimeline({ pipeline, tasks }: TaskTimelineProps) {
  const [now, setNow] = useState(Date.now);
  const [selection, setSelection] = useState<{
    taskId: string;
    stageId: string | null;
  } | null>(null);
  const hasActiveClock = pipeline.runs.some((run) =>
    run.cells.some((cell) => cell.runningSince !== null),
  );

  useEffect(() => {
    if (!hasActiveClock) return;
    setNow(Date.now());
    const interval = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(interval);
  }, [hasActiveClock]);

  const selectedRun = pipeline.runs.find(
    (run) => run.taskId === selection?.taskId,
  );
  const selectedStage = pipeline.stages.find(
    (stage) => stage.id === selection?.stageId,
  );
  const selectedCell = selectedRun?.cells.find(
    (cell) => cell.stageId === selection?.stageId,
  );
  const selectedTask = tasks.find((task) => task.id === selection?.taskId);
  let selectedEvents = selectedCell?.events ?? [];
  if (!selectedCell && selectedRun && selectedTask) {
    const taskEvents = selectedTask.attempts.map((attempt) => ({
      id: attempt.id,
      kind: "task" as const,
      code: attempt.message,
      message: attempt.message,
      at: attempt.startedAt!,
    }));
    const auditEvents = selectedRun.cells.flatMap((cell) =>
      cell.events.filter((event) => event.kind === "audit"),
    );
    selectedEvents = [
      ...new Map([...taskEvents, ...auditEvents].map((event) => [
        `${event.kind}-${event.id}`, event,
      ])).values(),
    ].sort((a, b) => new Date(a.at).getTime() - new Date(b.at).getTime());
  }

  return (
    <section className={styles.section} aria-labelledby="pipeline-heading">
      <header className={styles.header}>
        <h2 id="pipeline-heading" className={styles.title}>
          任务流水线
        </h2>
        <span className={styles.estimateNote}>预计：近期成功阶段均值</span>
      </header>
      {pipeline.runs.length === 0 ? (
        <EmptyState title="暂无执行记录" message="该发布单还未开始执行。" />
      ) : (
        <div
          className={styles.tableWrap}
          role="region"
          aria-label="流水线阶段表，可横向滚动"
          tabIndex={0}
        >
          <table className={styles.table} aria-labelledby="pipeline-heading">
            <thead>
              <tr>
                <th scope="col" className={styles.runHeader}>
                  执行
                </th>
                {pipeline.stages.map((stage) => (
                  <th scope="col" key={stage.id} className={styles.stageHeader}>
                    <span className={styles.stageName}>{stage.label}</span>
                    <span
                      className={styles.estimate}
                      title={
                        stage.kind === "automatic"
                          ? `最近成功部署中的 ${stage.sampleCount} 个有效样本`
                          : undefined
                      }
                    >
                      {stage.kind === "manual"
                        ? "人工"
                        : `预计 ${durationLabel(stage.estimatedDurationMs)}`}
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {pipeline.runs.map((run) => (
                <tr key={run.taskId}>
                  <th scope="row" className={styles.runInfo}>
                    <button
                      type="button"
                      className={styles.runNumber}
                      aria-label={`查看第 ${run.sequence} 次执行的任务日志`}
                      onClick={() =>
                        setSelection({ taskId: run.taskId, stageId: null })
                      }
                    >
                      第 {run.sequence} 次执行
                    </button>
                    <StatusPill
                      label={
                        run.status === 4 ? "部署成功" : taskStatusLabel(run.status)
                      }
                      tone={taskStatusTone(run.status)}
                    />
                    <time dateTime={run.startedAt} className={styles.runTime}>
                      {formatDateTime(run.startedAt)}
                    </time>
                    {run.replayCount > 0 ? (
                      <span className={styles.replayCount}>
                        重新派发 {run.replayCount} 次
                      </span>
                    ) : null}
                  </th>
                  {pipeline.stages.map((stage) => (
                    <td key={stage.id} className={styles.stageCell}>
                      <StageCell
                        cell={
                          run.cells.find((cell) => cell.stageId === stage.id)!
                        }
                        stage={stage}
                        sequence={run.sequence}
                        now={now}
                        onInspect={() =>
                          setSelection({ taskId: run.taskId, stageId: stage.id })
                        }
                      />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {selectedRun && selectedTask ? (
        <TaskLogModal
          task={selectedTask}
          taskName={`第 ${selectedRun.sequence} 次执行${selectedStage ? ` · ${selectedStage.label}` : ""}`}
          events={selectedEvents}
          onClose={() => setSelection(null)}
        />
      ) : null}
    </section>
  );
}
