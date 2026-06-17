import { useMemo } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { schedulerApi } from "../api";
import { servicesApi } from "../../services/api";
import { ActionButton } from "../../../shared/components/ActionButton";
import { isValidCron } from "./scheduler-utils";
import styles from "../../../styles/admin.module.css";
import { z } from "zod";

const cronPresets = [
  { label: "每5分钟", value: "*/5 * * * *" },
  { label: "每小时", value: "0 * * * *" },
  { label: "每天2点", value: "0 2 * * *" },
  { label: "每周日", value: "0 2 * * 0" },
  { label: "每月1日", value: "0 2 1 * *" },
];

const jobSchema = z
  .object({
    name: z.string().min(1, "名称必填").max(64, "最长 64 字符"),
    handlerKey: z.string().min(1, "Handler 标识符必填"),
    serviceId: z.string().uuid("请选择关联服务"),
    scheduleKind: z.enum(["one_time", "cron"]),
    cronExpr: z.string().optional(),
    runAt: z.string().optional(),
    dispatchPolicy: z.enum(["round_robin", "fixed_live_slot"]),
    leaseTimeoutSec: z.coerce.number().min(1).max(3600).default(60),
    maxRetries: z.coerce.number().min(0).max(10).default(3),
    payloadText: z.string().default("{}"),
    metadataText: z.string().default("{}"),
    enabled: z.boolean().default(true),
  })
  .refine(
    (data) => {
      if (data.scheduleKind === "cron") {
        return !!data.cronExpr && isValidCron(data.cronExpr);
      }
      return !!data.runAt && data.runAt.length > 0;
    },
    { message: "请填写有效的 Cron 表达式或执行时间", path: ["scheduleKind"] }
  )
  .refine(
    (data) => {
      try {
        const parsed = JSON.parse(data.payloadText);
        return typeof parsed === "object" && !Array.isArray(parsed);
      } catch {
        return false;
      }
    },
    { message: "Payload 必须是有效的 JSON 对象", path: ["payloadText"] }
  )
  .refine(
    (data) => {
      try {
        const parsed = JSON.parse(data.metadataText);
        return typeof parsed === "object" && !Array.isArray(parsed);
      } catch {
        return false;
      }
    },
    { message: "Metadata 必须是有效的 JSON 对象", path: ["metadataText"] }
  );

type JobFormData = z.infer<typeof jobSchema>;

export function JobForm() {
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
    reset,
    formState: { errors },
  } = useForm<JobFormData>({
    resolver: zodResolver(jobSchema) as any,
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

  const selectedService = useMemo(() => {
    return servicesQuery.data?.find((s) => s.id === serviceId) ?? null;
  }, [servicesQuery.data, serviceId]);

  const executorGroup = selectedService?.schedulerExecutorGroup ?? "";

  const createMutation = useMutation({
    mutationFn: schedulerApi.createJob,
    onSuccess: async () => {
      reset();
      await queryClient.invalidateQueries({ queryKey: ["scheduler", "jobs"] });
    },
  });

  const onSubmit = (data: JobFormData) => {
    const payload = JSON.parse(data.payloadText) as Record<string, unknown>;
    const metadata = JSON.parse(data.metadataText) as Record<string, string>;
    const runAt = data.runAt ? new Date(data.runAt).toISOString() : null;

    createMutation.mutate({
      name: data.name,
      handlerKey: data.handlerKey,
      serviceId: data.serviceId,
      payload,
      scheduleKind: data.scheduleKind,
      cronExpr: data.cronExpr,
      runAt,
      enabled: data.enabled,
      dispatchPolicy: data.dispatchPolicy,
      executorGroup,
      leaseTimeoutSec: data.leaseTimeoutSec,
      maxRetries: data.maxRetries,
      metadata,
    });
  };

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>新建任务</h2>
        </div>
        <ActionButton
          label="创建任务"
          variant="primary"
          pending={createMutation.isPending}
          onClick={handleSubmit(onSubmit)}
        />
      </div>
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          <span className={styles.label}>名称</span>
          <input className={styles.input} {...register("name")} />
          {errors.name && <span className={styles.inlineError}>{errors.name.message}</span>}
          <span className={styles.hint}>任务显示名称，如 "每日数据清理"</span>
        </label>

        <label className={styles.field}>
          <span className={styles.label}>Handler 标识符</span>
          <input className={styles.input} {...register("handlerKey")} placeholder="如 deploy、cleanup" />
          {errors.handlerKey && <span className={styles.inlineError}>{errors.handlerKey.message}</span>}
          <span className={styles.hint}>执行器端注册 Handler 时使用的路由标识</span>
        </label>

        <label className={styles.field}>
          <span className={styles.label}>关联服务</span>
          <select className={styles.select} {...register("serviceId")}>
            <option value="">请选择服务</option>
            {(servicesQuery.data ?? []).map((svc) => (
              <option key={svc.id} value={svc.id}>
                {svc.name}
              </option>
            ))}
          </select>
          {errors.serviceId && <span className={styles.inlineError}>{errors.serviceId.message}</span>}
          <span className={styles.hint}>调度任务将绑定到该服务的执行器组</span>
        </label>

        {selectedService && (
          <label className={styles.field}>
            <span className={styles.label}>执行器组</span>
            <input className={styles.input} value={executorGroup} readOnly disabled />
            <span className={styles.hint}>由关联服务自动填充</span>
          </label>
        )}

        <label className={styles.field}>
          <span className={styles.label}>调度方式</span>
          <select className={styles.select} {...register("scheduleKind")}>
            <option value="one_time">一次性</option>
            <option value="cron">Cron 周期</option>
          </select>
          {errors.scheduleKind && (
            <span className={styles.inlineError}>{errors.scheduleKind.message}</span>
          )}
          <span className={styles.hint}>一次性：仅执行一次；Cron：按周期重复执行</span>
        </label>

        {scheduleKind === "cron" ? (
          <label className={styles.field}>
            <span className={styles.label}>Cron 表达式</span>
            <input className={styles.input} {...register("cronExpr")} />
            <div className={styles.cronPresets}>
              {cronPresets.map((preset) => (
                <button
                  key={preset.value}
                  type="button"
                  className={styles.ghostButton}
                  onClick={() => setValue("cronExpr", preset.value)}
                >
                  {preset.label}
                </button>
              ))}
            </div>
            {errors.cronExpr && <span className={styles.inlineError}>{errors.cronExpr.message}</span>}
            <span className={styles.hint}>如 0 2 * * * 表示每天凌晨 2 点执行</span>
          </label>
        ) : (
          <label className={styles.field}>
            <span className={styles.label}>执行时间</span>
            <input className={styles.input} type="datetime-local" {...register("runAt")} />
            {errors.runAt && <span className={styles.inlineError}>{errors.runAt.message}</span>}
            <span className={styles.hint}>选择本地时间，自动转为 UTC ISO 提交</span>
          </label>
        )}

        <label className={styles.field}>
          <span className={styles.label}>分发策略</span>
          <select className={styles.select} {...register("dispatchPolicy")}>
            <option value="round_robin">轮询</option>
            <option value="fixed_live_slot">固定槽位</option>
          </select>
          <span className={styles.hint}>轮询：在可用执行器间轮询；固定槽位：绑定到当前服务蓝绿槽位</span>
        </label>

        <label className={styles.field}>
          <span className={styles.label}>租约超时（秒）</span>
          <input className={styles.input} type="number" {...register("leaseTimeoutSec")} />
          <span className={styles.hint}>任务执行最大等待时间，默认 60 秒</span>
        </label>

        <label className={styles.field}>
          <span className={styles.label}>最大重试次数</span>
          <input className={styles.input} type="number" {...register("maxRetries")} />
          <span className={styles.hint}>失败时自动重试次数，默认 3 次</span>
        </label>

        <label className={`${styles.field} ${styles.fieldWide}`}>
          <span className={styles.label}>任务载荷（JSON）</span>
          <textarea className={styles.textarea} rows={4} {...register("payloadText")} />
          {errors.payloadText && (
            <span className={styles.inlineError}>{errors.payloadText.message}</span>
          )}
          <span className={styles.hint}>
            任务执行时传入的参数对象，如 {'{"url": "https://example.com"}'}
          </span>
        </label>

        <label className={`${styles.field} ${styles.fieldWide}`}>
          <span className={styles.label}>元数据（JSON）</span>
          <textarea className={styles.textarea} rows={2} {...register("metadataText")} />
          {errors.metadataText && (
            <span className={styles.inlineError}>{errors.metadataText.message}</span>
          )}
          <span className={styles.hint}>
            任务附加标签，用于日志追踪，如 {'{"service_id": "abc"}'}
          </span>
        </label>
      </div>

      {(createMutation.error || Object.keys(errors).length > 0) && (
        <div className={styles.inlineError}>
          {createMutation.error ? getErrorMessage(createMutation.error) : "请修正表单错误"}
        </div>
      )}
    </section>
  );
}