import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import type { UpsertSchedulerExecutorInput } from "../types";
import { schedulerApi } from "../api";
import { ActionButton } from "../../../shared/components/ActionButton";
import styles from "../../../styles/admin.module.css";
import { newExecutorForm, parseMapJSON } from "./scheduler-utils";

export function ExecutorForm() {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<UpsertSchedulerExecutorInput>(() => newExecutorForm());
  const [metaText, setMetaText] = useState("{}");
  const [formError, setFormError] = useState("");

  const createMutation = useMutation({
    mutationFn: schedulerApi.createExecutor,
    onSuccess: async () => {
      setForm(newExecutorForm());
      setMetaText("{}");
      setFormError("");
      await queryClient.invalidateQueries({ queryKey: ["scheduler", "executors"] });
    },
    onError: (error) => setFormError(getErrorMessage(error)),
  });

  const handleCreate = () => {
    try {
      setFormError("");
      const metadata = parseMapJSON(metaText) as Record<string, string>;
      createMutation.mutate({ ...form, metadata });
    } catch (error) {
      setFormError(getErrorMessage(error));
    }
  };

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>新建执行器</h2>
        </div>
        <ActionButton label="创建执行器" variant="primary" pending={createMutation.isPending} onClick={handleCreate} />
      </div>
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          <span className={styles.label}>executorId</span>
          <input className={styles.input} value={form.executorId} onChange={(e) => setForm((v) => ({ ...v, executorId: e.target.value }))} />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>group</span>
          <input className={styles.input} value={form.group} onChange={(e) => setForm((v) => ({ ...v, group: e.target.value }))} />
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
