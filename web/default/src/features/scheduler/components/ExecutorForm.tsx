import { useMemo } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { schedulerApi } from "../api";
import { ActionButton } from "../../../shared/components/ActionButton";
import styles from "../../../styles/admin.module.css";
import { z } from "zod";

const executorSchema = z.object({
  executorId: z.string().min(1, "执行器 ID 必填").max(128, "最长 128 字符"),
  group: z.string().min(1, "组名必填").max(128, "最长 128 字符"),
  enabled: z.boolean().default(true),
  metadataText: z
    .string()
    .refine(
      (val) => {
        try {
          const parsed = JSON.parse(val);
          return typeof parsed === "object" && !Array.isArray(parsed);
        } catch {
          return false;
        }
      },
      { message: "Metadata 必须是有效的 JSON 对象" }
    )
    .default("{}"),
});

type ExecutorFormData = z.infer<typeof executorSchema>;

export function ExecutorForm() {
  const queryClient = useQueryClient();

  const executorsQuery = useQuery({
    queryKey: ["scheduler", "executors"],
    queryFn: schedulerApi.listExecutors,
  });

  const groups = useMemo(() => {
    const set = new Set(executorsQuery.data?.map((e) => e.group) ?? []);
    return Array.from(set).sort();
  }, [executorsQuery.data]);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ExecutorFormData>({
    resolver: zodResolver(executorSchema) as any,
    defaultValues: {
      executorId: "",
      group: "default",
      enabled: true,
      metadataText: "{}",
    },
  });

  const createMutation = useMutation({
    mutationFn: schedulerApi.createExecutor,
    onSuccess: async () => {
      reset();
      await queryClient.invalidateQueries({ queryKey: ["scheduler", "executors"] });
    },
  });

  const onSubmit = (data: ExecutorFormData) => {
    const metadata = JSON.parse(data.metadataText) as Record<string, string>;
    createMutation.mutate({
      executorId: data.executorId,
      group: data.group,
      enabled: data.enabled,
      metadata,
    });
  };

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>新建执行器</h2>
        </div>
        <ActionButton
          label="创建执行器"
          variant="primary"
          pending={createMutation.isPending}
          onClick={handleSubmit(onSubmit)}
        />
      </div>
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          <span className={styles.label}>执行器 ID</span>
          <input className={styles.input} {...register("executorId")} />
          {errors.executorId && (
            <span className={styles.inlineError}>{errors.executorId.message}</span>
          )}
          <span className={styles.hint}>唯一标识，如 executor-01</span>
        </label>

        <label className={styles.field}>
          <span className={styles.label}>组</span>
          <input className={styles.input} {...register("group")} list="executorGroups" />
          <datalist id="executorGroups">
            {groups.map((g) => (
              <option key={g} value={g} />
            ))}
          </datalist>
          {errors.group && <span className={styles.inlineError}>{errors.group.message}</span>}
          <span className={styles.hint}>执行器所属分组，如 default</span>
        </label>

        <label className={`${styles.field} ${styles.fieldWide}`}>
          <span className={styles.label}>元数据（JSON）</span>
          <textarea className={styles.textarea} rows={2} {...register("metadataText")} />
          {errors.metadataText && (
            <span className={styles.inlineError}>{errors.metadataText.message}</span>
          )}
          <span className={styles.hint}>
            执行器附加标签，用于日志追踪，如 {'{"region": "cn-north-1"}'}
          </span>
        </label>
      </div>

      {createMutation.error && (
        <div className={styles.inlineError}>{getErrorMessage(createMutation.error)}</div>
      )}
    </section>
  );
}
