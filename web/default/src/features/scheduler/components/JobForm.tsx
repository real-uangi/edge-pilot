import { useId, useMemo } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { schedulerApi } from "../api";
import { servicesApi } from "../../services/api";
import { ActionButton } from "../../../shared/components/ActionButton";
import {
  ErrorState,
  InlineNotice,
} from "../../../shared/components/StateBlocks";
import { isValidCron } from "./scheduler-utils";
import styles from "../../../styles/admin.module.css";
import { z } from "zod";

const cronPresets = [
  { label: "每 5 分钟", value: "*/5 * * * *" },
  { label: "每小时", value: "0 * * * *" },
  { label: "每天 2 点", value: "0 2 * * *" },
  { label: "每周日", value: "0 2 * * 0" },
  { label: "每月 1 日", value: "0 2 1 * *" },
];
const jobSchema = z
  .object({
    name: z.string().trim().min(1, "请输入任务名称").max(64, "最长 64 字符"),
    handlerKey: z.string().trim().min(1, "请输入 Handler 标识符"),
    serviceId: z.string().uuid("请选择关联服务"),
    scheduleKind: z.enum(["one_time", "cron"]),
    cronExpr: z.string().optional(),
    runAt: z.string().optional(),
    dispatchPolicy: z.enum(["round_robin", "fixed_live_slot"]),
    leaseTimeoutSec: z.coerce
      .number()
      .int("请输入整数")
      .min(1, "至少 1 秒")
      .max(3600, "最多 3600 秒")
      .default(60),
    maxRetries: z.coerce
      .number()
      .int("请输入整数")
      .min(0, "不能小于 0")
      .max(10, "最多重试 10 次")
      .default(3),
    payloadText: z.string().default("{}"),
    metadataText: z.string().default("{}"),
    enabled: z.boolean().default(true),
  })
  .superRefine((data, context) => {
    if (
      data.scheduleKind === "cron" &&
      (!data.cronExpr || !isValidCron(data.cronExpr))
    )
      context.addIssue({
        code: "custom",
        message: "请输入有效的 Cron 表达式",
        path: ["cronExpr"],
      });
    if (
      data.scheduleKind === "one_time" &&
      (!data.runAt || Number.isNaN(new Date(data.runAt).getTime()))
    )
      context.addIssue({
        code: "custom",
        message: "请选择执行时间",
        path: ["runAt"],
      });
    for (const field of ["payloadText", "metadataText"] as const) {
      try {
        const parsed = JSON.parse(data[field]);
        if (
          parsed === null ||
          typeof parsed !== "object" ||
          Array.isArray(parsed)
        )
          context.addIssue({
            code: "custom",
            message: "请输入 JSON 对象",
            path: [field],
          });
        else if (
          field === "metadataText" &&
          !Object.values(parsed).every((value) => typeof value === "string")
        )
          context.addIssue({
            code: "custom",
            message: "元数据的值必须为字符串",
            path: [field],
          });
      } catch {
        context.addIssue({
          code: "custom",
          message: "请输入有效的 JSON 对象",
          path: [field],
        });
      }
    }
  });
type JobFormData = z.output<typeof jobSchema>;

export function JobForm({
  onCreated,
  onCancel,
}: {
  onCreated: () => void;
  onCancel: () => void;
}) {
  const fieldId = useId();
  const queryClient = useQueryClient();
  const servicesQuery = useQuery({
    queryKey: ["services"],
    queryFn: servicesApi.list,
  });
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    setError,
    formState: { errors },
  } = useForm<z.input<typeof jobSchema>, unknown, JobFormData>({
    resolver: zodResolver(jobSchema),
    defaultValues: {
      name: "",
      handlerKey: "",
      serviceId: "",
      scheduleKind: "one_time",
      cronExpr: "*/5 * * * *",
      runAt: "",
      dispatchPolicy: "round_robin",
      leaseTimeoutSec: 60,
      maxRetries: 3,
      payloadText: "{}",
      metadataText: "{}",
      enabled: true,
    },
  });
  const scheduleKind = watch("scheduleKind");
  const serviceId = watch("serviceId");
  const selectedService = useMemo(
    () => servicesQuery.data?.find((service) => service.id === serviceId),
    [servicesQuery.data, serviceId],
  );
  const createMutation = useMutation({
    mutationKey: ["scheduler", "create-job"],
    mutationFn: schedulerApi.createJob,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["scheduler", "jobs"] });
      onCreated();
    },
  });
  const onSubmit = (data: JobFormData) => {
    if (!selectedService) {
      setError(
        "serviceId",
        { message: "请选择可用的关联服务" },
        { shouldFocus: true },
      );
      return;
    }
    createMutation.mutate({
      name: data.name,
      handlerKey: data.handlerKey,
      serviceId: data.serviceId,
      payload: JSON.parse(data.payloadText) as Record<string, unknown>,
      scheduleKind: data.scheduleKind,
      cronExpr: data.cronExpr,
      runAt: data.runAt ? new Date(data.runAt).toISOString() : null,
      enabled: data.enabled,
      dispatchPolicy: data.dispatchPolicy,
      executorGroup: selectedService.schedulerExecutorGroup,
      leaseTimeoutSec: data.leaseTimeoutSec,
      maxRetries: data.maxRetries,
      metadata: JSON.parse(data.metadataText) as Record<string, string>,
    });
  };

  return (
    <form
      className={styles.formStack}
      onSubmit={handleSubmit(onSubmit)}
      noValidate
    >
      <section className={styles.formSection}>
        <h3>基本信息</h3>
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>名称 *</span>
            <input
              className={styles.input}
              {...register("name")}
              aria-required="true"
              aria-invalid={Boolean(errors.name)}
              aria-describedby={
                errors.name ? `${fieldId}-name-error` : undefined
              }
            />
            {errors.name ? (
              <span className={styles.fieldError} id={`${fieldId}-name-error`}>
                {errors.name.message}
              </span>
            ) : null}
          </label>
          <label className={styles.field}>
            <span className={styles.label}>Handler 标识符 *</span>
            <input
              className={styles.input}
              {...register("handlerKey")}
              aria-required="true"
              aria-invalid={Boolean(errors.handlerKey)}
              aria-describedby={
                errors.handlerKey ? `${fieldId}-handler-error` : undefined
              }
            />
            {errors.handlerKey ? (
              <span
                className={styles.fieldError}
                id={`${fieldId}-handler-error`}
              >
                {errors.handlerKey.message}
              </span>
            ) : null}
          </label>
          <label className={styles.field}>
            <span className={styles.label}>关联服务 *</span>
            <select
              className={styles.select}
              {...register("serviceId")}
              disabled={servicesQuery.isPending || !servicesQuery.data?.length}
              aria-required="true"
              aria-invalid={Boolean(errors.serviceId)}
              aria-describedby={
                errors.serviceId ? `${fieldId}-service-error` : undefined
              }
            >
              <option value="">
                {servicesQuery.isPending
                  ? "正在加载服务"
                  : !servicesQuery.data?.length
                    ? "暂无可选服务"
                    : "请选择服务"}
              </option>
              {servicesQuery.data?.map((service) => (
                <option key={service.id} value={service.id}>
                  {service.name}
                </option>
              ))}
            </select>
            {errors.serviceId ? (
              <span
                className={styles.fieldError}
                id={`${fieldId}-service-error`}
              >
                {errors.serviceId.message}
              </span>
            ) : null}
          </label>
          {selectedService ? (
            <div className={styles.field}>
              <span className={styles.label}>执行器组</span>
              <code>{selectedService.schedulerExecutorGroup}</code>
            </div>
          ) : null}
        </div>
        {servicesQuery.isError ? (
          <ErrorState
            title="服务列表加载失败"
            message={getErrorMessage(servicesQuery.error)}
            onRetry={() => void servicesQuery.refetch()}
          />
        ) : null}
      </section>
      <section className={styles.formSection}>
        <h3>执行时间</h3>
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>调度方式</span>
            <select className={styles.select} {...register("scheduleKind")}>
              <option value="one_time">一次性</option>
              <option value="cron">Cron 周期</option>
            </select>
          </label>
          {scheduleKind === "cron" ? (
            <div className={styles.field}>
              <label className={styles.label} htmlFor={`${fieldId}-cron`}>
                Cron 表达式 *
              </label>
              <input
                id={`${fieldId}-cron`}
                className={styles.input}
                {...register("cronExpr")}
                aria-required="true"
                aria-invalid={Boolean(errors.cronExpr)}
                aria-describedby={
                  errors.cronExpr ? `${fieldId}-cron-error` : undefined
                }
              />
              <div className={styles.buttonRow}>
                {cronPresets.map((preset) => (
                  <button
                    type="button"
                    className={styles.ghostButton}
                    key={preset.value}
                    onClick={() =>
                      setValue("cronExpr", preset.value, {
                        shouldValidate: true,
                        shouldDirty: true,
                      })
                    }
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
              {errors.cronExpr ? (
                <span
                  className={styles.fieldError}
                  id={`${fieldId}-cron-error`}
                >
                  {errors.cronExpr.message}
                </span>
              ) : null}
            </div>
          ) : (
            <label className={styles.field}>
              <span className={styles.label}>执行时间 *</span>
              <input
                className={styles.input}
                type="datetime-local"
                {...register("runAt")}
                aria-required="true"
                aria-invalid={Boolean(errors.runAt)}
                aria-describedby={`${fieldId}-time-hint${errors.runAt ? ` ${fieldId}-time-error` : ""}`}
              />
              <span className={styles.hint} id={`${fieldId}-time-hint`}>
                本地时区：{Intl.DateTimeFormat().resolvedOptions().timeZone}
              </span>
              {errors.runAt ? (
                <span
                  className={styles.fieldError}
                  id={`${fieldId}-time-error`}
                >
                  {errors.runAt.message}
                </span>
              ) : null}
            </label>
          )}
        </div>
      </section>
      <section className={styles.formSection}>
        <h3>执行参数</h3>
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>分发策略</span>
            <select className={styles.select} {...register("dispatchPolicy")}>
              <option value="round_robin">轮询</option>
              <option value="fixed_live_slot">固定槽位</option>
            </select>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>租约超时（秒）</span>
            <input
              className={styles.input}
              type="number"
              min={1}
              max={3600}
              {...register("leaseTimeoutSec")}
              aria-invalid={Boolean(errors.leaseTimeoutSec)}
              aria-describedby={
                errors.leaseTimeoutSec ? `${fieldId}-lease-error` : undefined
              }
            />
            {errors.leaseTimeoutSec ? (
              <span className={styles.fieldError} id={`${fieldId}-lease-error`}>
                {errors.leaseTimeoutSec.message}
              </span>
            ) : null}
          </label>
          <label className={styles.field}>
            <span className={styles.label}>最大重试次数</span>
            <input
              className={styles.input}
              type="number"
              min={0}
              max={10}
              {...register("maxRetries")}
              aria-invalid={Boolean(errors.maxRetries)}
              aria-describedby={
                errors.maxRetries ? `${fieldId}-retries-error` : undefined
              }
            />
            {errors.maxRetries ? (
              <span
                className={styles.fieldError}
                id={`${fieldId}-retries-error`}
              >
                {errors.maxRetries.message}
              </span>
            ) : null}
          </label>
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>任务载荷（JSON）</span>
            <textarea
              className={styles.textarea}
              rows={4}
              {...register("payloadText")}
              spellCheck={false}
              aria-invalid={Boolean(errors.payloadText)}
              aria-describedby={
                errors.payloadText ? `${fieldId}-payload-error` : undefined
              }
            />
            {errors.payloadText ? (
              <span
                className={styles.fieldError}
                id={`${fieldId}-payload-error`}
              >
                {errors.payloadText.message}
              </span>
            ) : null}
          </label>
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>元数据（JSON）</span>
            <textarea
              className={styles.textarea}
              rows={2}
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
      </section>
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
          label="创建任务"
          type="submit"
          variant="primary"
          pending={createMutation.isPending}
          pendingLabel="创建中"
          disabled={!servicesQuery.data?.length}
        />
      </div>
    </form>
  );
}
