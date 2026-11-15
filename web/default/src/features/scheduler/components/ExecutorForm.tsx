import { useId, useMemo } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { schedulerApi } from "../api";
import type {
  SchedulerExecutorRecord,
  UpsertSchedulerExecutorInput,
} from "../types";
import { ActionButton } from "../../../shared/components/ActionButton";
import { InlineNotice } from "../../../shared/components/StateBlocks";
import styles from "../../../styles/admin.module.css";
import { z } from "zod";

const executorSchema = z.object({
  executorId: z
    .string()
    .trim()
    .min(1, "请输入执行器 ID")
    .max(128, "最长 128 字符"),
  group: z.string().trim().min(1, "请输入组名").max(128, "最长 128 字符"),
  enabled: z.boolean().default(true),
  metadataText: z
    .string()
    .refine((value) => {
      try {
        const parsed = JSON.parse(value);
        return (
          parsed !== null &&
          typeof parsed === "object" &&
          !Array.isArray(parsed) &&
          Object.values(parsed).every((item) => typeof item === "string")
        );
      } catch {
        return false;
      }
    }, "请输入值为字符串的 JSON 对象")
    .default("{}"),
});
type ExecutorFormData = z.output<typeof executorSchema>;

export function ExecutorForm({
  onCreated,
  onCancel,
}: {
  onCreated: (output: SchedulerExecutorRecord) => void;
  onCancel: () => void;
}) {
  const fieldId = useId();
  const queryClient = useQueryClient();
  const executorsQuery = useQuery({
    queryKey: ["scheduler", "executors"],
    queryFn: schedulerApi.listExecutors,
  });
  const groups = useMemo(
    () =>
      Array.from(
        new Set(executorsQuery.data?.map((executor) => executor.group) ?? []),
      ).sort(),
    [executorsQuery.data],
  );
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<z.input<typeof executorSchema>, unknown, ExecutorFormData>({
    resolver: zodResolver(executorSchema),
    defaultValues: {
      executorId: "",
      group: "default",
      enabled: true,
      metadataText: "{}",
    },
  });
  const createMutation = useMutation({
    mutationKey: ["scheduler", "create-executor"],
    mutationFn: async (input: UpsertSchedulerExecutorInput) => {
      const output = await schedulerApi.createExecutor(input);
      await queryClient.invalidateQueries({
        queryKey: ["scheduler", "executors"],
      });
      onCreated(output);
    },
  });
  const onSubmit = (data: ExecutorFormData) =>
    createMutation.mutate({
      executorId: data.executorId,
      group: data.group,
      enabled: data.enabled,
      metadata: JSON.parse(data.metadataText) as Record<string, string>,
    });

  return (
    <form
      className={styles.formStack}
      onSubmit={handleSubmit(onSubmit)}
      noValidate
    >
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          <span className={styles.label}>
            执行器 ID <span aria-hidden="true">*</span>
          </span>
          <input
            className={styles.input}
            {...register("executorId")}
            autoComplete="off"
            aria-required="true"
            aria-invalid={Boolean(errors.executorId)}
            aria-describedby={
              errors.executorId ? `${fieldId}-id-error` : undefined
            }
          />
          {errors.executorId ? (
            <span className={styles.fieldError} id={`${fieldId}-id-error`}>
              {errors.executorId.message}
            </span>
          ) : null}
        </label>
        <label className={styles.field}>
          <span className={styles.label}>
            组 <span aria-hidden="true">*</span>
          </span>
          <input
            className={styles.input}
            {...register("group")}
            list={`${fieldId}-groups`}
            aria-required="true"
            aria-invalid={Boolean(errors.group)}
            aria-describedby={
              errors.group ? `${fieldId}-group-error` : undefined
            }
          />
          <datalist id={`${fieldId}-groups`}>
            {groups.map((group) => (
              <option key={group} value={group} />
            ))}
          </datalist>
          {errors.group ? (
            <span className={styles.fieldError} id={`${fieldId}-group-error`}>
              {errors.group.message}
            </span>
          ) : null}
        </label>
        <label className={`${styles.field} ${styles.fieldWide}`}>
          <span className={styles.label}>元数据（JSON）</span>
          <textarea
            className={styles.textarea}
            rows={4}
            {...register("metadataText")}
            spellCheck={false}
            aria-invalid={Boolean(errors.metadataText)}
            aria-describedby={
              errors.metadataText ? `${fieldId}-metadata-error` : undefined
            }
          />
          {errors.metadataText ? (
            <span
              className={styles.fieldError}
              id={`${fieldId}-metadata-error`}
            >
              {errors.metadataText.message}
            </span>
          ) : null}
        </label>
      </div>
      {createMutation.isError ? (
        <InlineNotice message={getErrorMessage(createMutation.error)} />
      ) : null}
      <div className={styles.formActions}>
        <ActionButton
          label="取消"
          disabled={createMutation.isPending}
          onClick={onCancel}
        />
        <ActionButton
          label="创建执行器"
          variant="primary"
          type="submit"
          pending={createMutation.isPending}
          pendingLabel="创建中"
        />
      </div>
    </form>
  );
}
