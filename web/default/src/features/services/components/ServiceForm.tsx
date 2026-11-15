import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatAgentLabel } from "../../../shared/lib/format";
import {
  serviceFormSchema,
  toServiceFormDefaults,
  type ServiceFormInput,
  type ServiceFormValues,
} from "../../../shared/lib/forms";
import { ActionButton } from "../../../shared/components/ActionButton";
import { InlineNotice } from "../../../shared/components/StateBlocks";
import { agentsApi } from "../../agents/api";
import type { ServiceRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface ServiceFormProps {
  service?: ServiceRecord;
  isEdit: boolean;
  submitError: string | null;
  savePending: boolean;
  deletePending: boolean;
  onSubmit: (values: ServiceFormValues) => Promise<ServiceRecord>;
  onDelete: () => void;
  onCancel: () => void;
}

export function ServiceForm({
  service,
  isEdit,
  submitError,
  savePending,
  deletePending,
  onSubmit,
  onDelete,
  onCancel,
}: ServiceFormProps) {
  const [saved, setSaved] = useState(false);
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: agentsApi.list,
  });
  const form = useForm<ServiceFormInput, unknown, ServiceFormValues>({
    resolver: zodResolver(serviceFormSchema),
    defaultValues: toServiceFormDefaults(service),
  });
  const busy = savePending || deletePending;
  const { errors, isDirty } = form.formState;

  const field = (
    name: keyof ServiceFormInput,
    label: string,
    kind: "text" | "number" | "textarea" = "text",
    hint?: string,
    wide = false,
  ) => {
    const error = errors[name];
    const id = `service-${name}`;
    const accessibility = {
      id,
      "aria-invalid": Boolean(error),
      "aria-describedby":
        [hint ? `${id}-hint` : "", error ? `${id}-error` : ""]
          .filter(Boolean)
          .join(" ") || undefined,
    };
    return (
      <label
        className={`${styles.field} ${wide ? styles.fieldWide : ""}`}
        key={name}
      >
        <span className={styles.label}>{label}</span>
        {kind === "textarea" ? (
          <textarea
            className={styles.textarea}
            {...form.register(name)}
            {...accessibility}
          />
        ) : (
          <input
            className={styles.input}
            type={kind}
            step={name === "cpuLimitCores" ? "0.1" : undefined}
            readOnly={name === "serviceKey" && isEdit}
            {...form.register(name)}
            {...accessibility}
          />
        )}
        {hint ? (
          <span className={styles.hint} id={`${id}-hint`}>
            {hint}
          </span>
        ) : null}
        {error ? (
          <span className={styles.fieldError} id={`${id}-error`}>
            {error.message}
          </span>
        ) : null}
      </label>
    );
  };

  return (
    <form
      className={styles.page}
      noValidate
      onSubmit={form.handleSubmit((values) =>
        onSubmit(values).then(
          (updated) => {
            form.reset(toServiceFormDefaults(updated));
            setSaved(true);
          },
          () => undefined,
        ),
      )}
    >
      <fieldset className={styles.formSection} disabled={busy}>
        <legend className={styles.formLegend}>基础信息</legend>
        <div className={styles.fieldGrid}>
          {field("name", "名称")}
          {field(
            "serviceKey",
            "服务标识",
            "text",
            isEdit ? "创建后不可修改" : undefined,
          )}
          <label className={styles.field}>
            <span className={styles.label}>节点</span>
            <select
              className={styles.select}
              disabled={isEdit || agentsQuery.isPending}
              {...form.register("agentId")}
              aria-invalid={Boolean(errors.agentId)}
              aria-describedby={
                errors.agentId ? "service-agentId-error" : undefined
              }
            >
              <option value="">
                {agentsQuery.isPending ? "正在加载节点" : "选择节点"}
              </option>
              {agentsQuery.data?.map((agent) => (
                <option key={agent.id} value={agent.id}>
                  {formatAgentLabel(agent)}
                </option>
              ))}
            </select>
            {isEdit ? (
              <span className={styles.hint}>创建后不可修改</span>
            ) : null}
            {errors.agentId ? (
              <span className={styles.fieldError} id="service-agentId-error">
                {errors.agentId.message}
              </span>
            ) : null}
          </label>
          {field("imageRepo", "镜像仓库")}
          {field("containerPort", "容器端口", "number")}
          <label className={styles.checkboxRow}>
            <input type="checkbox" {...form.register("enabled")} />
            <span className={styles.label}>启用服务</span>
          </label>
        </div>
        {agentsQuery.isError ? (
          <div className={styles.inlineRow}>
            <InlineNotice
              message={`节点加载失败：${getErrorMessage(agentsQuery.error)}`}
              tone="error"
            />
            <ActionButton
              label="重试"
              onClick={() => void agentsQuery.refetch()}
              pending={agentsQuery.isFetching}
              variant="ghost"
            />
          </div>
        ) : null}
      </fieldset>

      <fieldset className={styles.formSection} disabled={busy}>
        <legend className={styles.formLegend}>路由与网络</legend>
        <div className={styles.fieldGrid}>
          {field("routeHost", "主域名")}
          {field("routePathPrefix", "路由前缀")}
          {field(
            "routeHostsText",
            "接入域名",
            "textarea",
            "每行一个域名，需包含主域名",
            true,
          )}
          {field(
            "networkAliasesText",
            "网络别名",
            "textarea",
            "每行一个 RFC1123 小写别名",
          )}
          {field(
            "publishedPortsText",
            "暴露端口",
            "textarea",
            "每行一个 host:container",
          )}
          {field(
            "tcpProxyPortsText",
            "TCP 代理端口",
            "textarea",
            "每行一个 公网端口:容器端口[:空闲秒]，默认 3600 秒",
            true,
          )}
        </div>
      </fieldset>

      <fieldset className={styles.formSection} disabled={busy}>
        <legend className={styles.formLegend}>健康检查</legend>
        <div className={styles.fieldGrid}>
          <label className={`${styles.checkboxRow} ${styles.fieldWide}`}>
            <input type="checkbox" {...form.register("dockerHealthCheck")} />
            <span className={styles.label}>启用 Docker 探活</span>
          </label>
          {field(
            "httpHealthPath",
            "HTTP 探活路径",
            "text",
            "留空则不进行 HTTP 探活",
          )}
          {field("httpExpectedCode", "预期状态码", "number")}
          {field(
            "httpHealthHeadersText",
            "HTTP 探活 Header",
            "textarea",
            "每行一个 Header-Name: value",
            true,
          )}
          {field("httpTimeoutSecond", "预热窗口（秒）", "number")}
          {field("startupGraceSecond", "启动宽限（秒）", "number")}
          {field("httpProbeTimeoutSecond", "单次探测超时（秒）", "number")}
          {field("httpProbeIntervalSecond", "探测间隔（秒）", "number")}
          {field("httpSuccessThreshold", "连续成功阈值", "number")}
        </div>
      </fieldset>

      <fieldset className={styles.formSection} disabled={busy}>
        <legend className={styles.formLegend}>运行参数</legend>
        <div className={styles.fieldGrid}>
          {field("cpuLimitCores", "CPU 限制（核）", "number", "0 表示不限制")}
          {field("memoryLimitMB", "内存限制（MB）", "number", "0 表示不限制")}
          {field("envText", "环境变量", "textarea", "每行一个 KEY=value", true)}
          {field("commandText", "命令", "textarea", "每行一个参数")}
          {field("entrypointText", "入口命令", "textarea", "每行一个参数")}
          {field(
            "volumesText",
            "挂载卷",
            "textarea",
            "每行一个 /src:/dst[:ro]",
            true,
          )}
        </div>
      </fieldset>

      <fieldset className={styles.formSection} disabled={busy}>
        <legend className={styles.formLegend}>调度执行器</legend>
        <div className={styles.fieldGrid}>
          {field("schedulerSdkPort", "SDK 端口", "number")}
          {field("schedulerSdkAddr", "SDK 地址", "text", "默认 127.0.0.1")}
          {field("schedulerExecutorGroup", "执行器组")}
        </div>
      </fieldset>

      {submitError ? <InlineNotice message={submitError} tone="error" /> : null}
      <div className={styles.formActions}>
        <div className={styles.buttonRow}>
          <ActionButton
            label={isEdit ? "保存配置" : "创建服务"}
            pending={savePending}
            pendingLabel="保存中"
            disabled={busy || (isEdit && !isDirty)}
            type="submit"
            variant="primary"
          />
          <ActionButton
            label={isEdit ? "返回概览" : "取消"}
            onClick={onCancel}
            variant="ghost"
          />
          <span
            className={isDirty ? styles.hint : styles.successText}
            role="status"
          >
            {isDirty ? "有未保存的更改" : saved ? "已保存" : ""}
          </span>
        </div>
        {isEdit ? (
          <ActionButton
            label="删除服务"
            pending={deletePending}
            disabled={busy}
            onClick={onDelete}
            variant="danger"
          />
        ) : null}
      </div>
    </form>
  );
}
