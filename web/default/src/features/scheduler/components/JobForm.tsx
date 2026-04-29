import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import type { UpsertSchedulerJobInput } from "../types";
import { schedulerApi } from "../api";
import { ActionButton } from "../../../shared/components/ActionButton";
import styles from "../../../styles/admin.module.css";
import { newJobForm, parseMapJSON } from "./scheduler-utils";

export function JobForm() {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<UpsertSchedulerJobInput>(() => newJobForm());
  const [payloadText, setPayloadText] = useState("{}");
  const [metaText, setMetaText] = useState("{}");
  const [formError, setFormError] = useState("");

  const createMutation = useMutation({
    mutationFn: schedulerApi.createJob,
    onSuccess: async () => {
      setForm(newJobForm());
      setPayloadText("{}");
      setMetaText("{}");
      setFormError("");
      await queryClient.invalidateQueries({ queryKey: ["scheduler", "jobs"] });
    },
    onError: (error) => setFormError(getErrorMessage(error)),
  });

  const handleCreate = () => {
    try {
      setFormError("");
      const payload = parseMapJSON(payloadText);
      const metadata = parseMapJSON(metaText) as Record<string, string>;
      createMutation.mutate({ ...form, payload, metadata });
    } catch (error) {
      setFormError(getErrorMessage(error));
    }
  };

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>新建任务</h2>
        </div>
        <ActionButton label="创建任务" variant="primary" pending={createMutation.isPending} onClick={handleCreate} />
      </div>
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          <span className={styles.label}>名称</span>
          <input className={styles.input} value={form.name} onChange={(e) => setForm((v) => ({ ...v, name: e.target.value }))} />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>taskType</span>
          <input className={styles.input} value={form.taskType} onChange={(e) => setForm((v) => ({ ...v, taskType: e.target.value }))} />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>scheduleKind</span>
          <select className={styles.select} value={form.scheduleKind} onChange={(e) => setForm((v) => ({ ...v, scheduleKind: e.target.value as "one_time" | "cron" }))}>
            <option value="one_time">one_time</option>
            <option value="cron">cron</option>
          </select>
        </label>
        <label className={styles.field}>
          <span className={styles.label}>dispatchPolicy</span>
          <select className={styles.select} value={form.dispatchPolicy ?? "round_robin"} onChange={(e) => setForm((v) => ({ ...v, dispatchPolicy: e.target.value as "round_robin" | "fixed_live_slot" }))}>
            <option value="round_robin">round_robin</option>
            <option value="fixed_live_slot">fixed_live_slot</option>
          </select>
        </label>
        <label className={styles.field}>
          <span className={styles.label}>executorGroup</span>
          <input className={styles.input} value={form.executorGroup} onChange={(e) => setForm((v) => ({ ...v, executorGroup: e.target.value }))} />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>leaseTimeoutSec</span>
          <input className={styles.input} type="number" value={form.leaseTimeoutSec ?? 60} onChange={(e) => setForm((v) => ({ ...v, leaseTimeoutSec: Number(e.target.value) || 60 }))} />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>maxRetries</span>
          <input className={styles.input} type="number" value={form.maxRetries ?? 3} onChange={(e) => setForm((v) => ({ ...v, maxRetries: Number(e.target.value) || 0 }))} />
        </label>
        {form.scheduleKind === "cron" ? (
          <label className={styles.field}>
            <span className={styles.label}>cronExpr</span>
            <input className={styles.input} value={form.cronExpr ?? ""} onChange={(e) => setForm((v) => ({ ...v, cronExpr: e.target.value }))} />
          </label>
        ) : (
          <label className={styles.field}>
            <span className={styles.label}>runAt (UTC ISO)</span>
            <input className={styles.input} value={form.runAt ?? ""} onChange={(e) => setForm((v) => ({ ...v, runAt: e.target.value }))} />
          </label>
        )}
        <label className={`${styles.field} ${styles.fieldWide}`}>
          <span className={styles.label}>payload(JSON)</span>
          <textarea className={styles.textarea} rows={4} value={payloadText} onChange={(e) => setPayloadText(e.target.value)} />
        </label>
        <label className={`${styles.field} ${styles.fieldWide}`}>
          <span className={styles.label}>metadata(JSON)</span>
          <textarea className={styles.textarea} rows={2} value={metaText} onChange={(e) => setMetaText(e.target.value)} />
        </label>
      </div>
      {formError ? <div className={styles.inlineError}>{formError}</div> : null}
    </section>
  );
}
