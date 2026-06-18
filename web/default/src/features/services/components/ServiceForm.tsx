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
  onSubmit: (values: ServiceFormValues) => void;
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
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: agentsApi.list,
  });

  const form = useForm<ServiceFormInput, unknown, ServiceFormValues>({
    resolver: zodResolver(serviceFormSchema),
    defaultValues: toServiceFormDefaults(service),
  });

  return (
    <section className={styles.sectionCard}>
      {agentsQuery.isError ? (
        <InlineNotice message={`节点列表加载失败：${getErrorMessage(agentsQuery.error)}`} tone="error" />
      ) : null}

      <form
        onSubmit={form.handleSubmit((values) => {
          onSubmit(values);
        })}
      >
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>名称</span>
            <input className={styles.input} {...form.register("name")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>服务标识</span>
            <input className={styles.input} readOnly={isEdit} {...form.register("serviceKey")} />
            {isEdit ? <span className={styles.hint}>创建后不可修改</span> : null}
          </label>
          <label className={styles.field}>
            <span className={styles.label}>节点</span>
            <select className={styles.select} disabled={isEdit} {...form.register("agentId")}>
              <option value="">选择节点</option>
              {agentsQuery.data?.map((agent) => (
                <option key={agent.id} value={agent.id}>
                  {formatAgentLabel(agent)}
                </option>
              ))}
            </select>
            {isEdit ? <span className={styles.hint}>创建后不可修改</span> : null}
          </label>
          <label className={styles.field}>
            <span className={styles.label}>镜像仓库</span>
            <input className={styles.input} {...form.register("imageRepo")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>容器端口</span>
            <input className={styles.input} type="number" {...form.register("containerPort")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>CPU 限制（核）</span>
            <input className={styles.input} type="number" step="0.1" min="0" {...form.register("cpuLimitCores")} />
            <span className={styles.hint}>默认 0，不限制</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>内存限制（MB）</span>
            <input className={styles.input} type="number" min="0" {...form.register("memoryLimitMB")} />
            <span className={styles.hint}>默认 0，不限制</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>主域名</span>
            <input className={styles.input} {...form.register("routeHost")} />
          </label>
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>接入域名</span>
            <textarea className={styles.textarea} {...form.register("routeHostsText")} />
            <span className={styles.hint}>每行一个域名，需包含主域名</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>路由前缀</span>
            <input className={styles.input} {...form.register("routePathPrefix")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>HTTP 探活路径</span>
            <input className={styles.input} {...form.register("httpHealthPath")} />
          </label>
          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>HTTP 探活 Header</span>
            <textarea className={styles.textarea} {...form.register("httpHealthHeadersText")} />
            <span className={styles.hint}>每行一个：`Header-Name: value`</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>预期状态码</span>
            <input className={styles.input} type="number" {...form.register("httpExpectedCode")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>超时秒数</span>
            <input className={styles.input} type="number" {...form.register("httpTimeoutSecond")} />
            <span className={styles.hint}>总预热窗口，默认 90 秒</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>启动宽限秒数</span>
            <input className={styles.input} type="number" {...form.register("startupGraceSecond")} />
            <span className={styles.hint}>容器启动后先等待，再进入探测循环</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>单次探测超时</span>
            <input className={styles.input} type="number" {...form.register("httpProbeTimeoutSecond")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>探测间隔秒数</span>
            <input className={styles.input} type="number" {...form.register("httpProbeIntervalSecond")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>连续成功阈值</span>
            <input className={styles.input} type="number" {...form.register("httpSuccessThreshold")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>调度 SDK 端口</span>
            <input className={styles.input} type="number" {...form.register("schedulerSdkPort")} />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>调度 SDK 地址</span>
            <input className={styles.input} {...form.register("schedulerSdkAddr")} placeholder="127.0.0.1" />
            <span className={styles.hint}>默认 127.0.0.1，容器内 SDK 连接 scheduler 的地址</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>执行器组</span>
            <input className={styles.input} {...form.register("schedulerExecutorGroup")} />
          </label>
          <label className={styles.field}>
            <span className={styles.checkboxRow}>
              <input type="checkbox" {...form.register("dockerHealthCheck")} />
              <span className={styles.label}>启用 Docker 探活</span>
            </span>
          </label>
          <label className={styles.field}>
            <span className={styles.checkboxRow}>
              <input type="checkbox" {...form.register("enabled")} />
              <span className={styles.label}>启用服务</span>
            </span>
          </label>

          <label className={`${styles.field} ${styles.fieldWide}`}>
            <span className={styles.label}>环境变量</span>
            <textarea className={styles.textarea} {...form.register("envText")} />
            <span className={styles.hint}>每行一个：`KEY=value`</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>命令</span>
            <textarea className={styles.textarea} {...form.register("commandText")} />
            <span className={styles.hint}>每行一个参数</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>入口命令</span>
            <textarea className={styles.textarea} {...form.register("entrypointText")} />
            <span className={styles.hint}>每行一个参数</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>挂载卷</span>
            <textarea className={styles.textarea} {...form.register("volumesText")} />
            <span className={styles.hint}>格式：`/src:/dst[:ro]`</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>网络别名</span>
            <textarea className={styles.textarea} {...form.register("networkAliasesText")} />
            <span className={styles.hint}>每行一个 RFC1123 小写别名，如：`svc-api`</span>
          </label>
          <label className={styles.field}>
            <span className={styles.label}>暴露端口</span>
            <textarea className={styles.textarea} {...form.register("publishedPortsText")} />
            <span className={styles.hint}>格式：`host:container`</span>
          </label>
        </div>

        {submitError ? <InlineNotice message={submitError} tone="error" /> : null}

        <div className={styles.buttonRow} style={{ marginTop: 24 }}>
          <ActionButton
            label={isEdit ? "更新服务" : "创建服务"}
            pending={savePending}
            pendingLabel="保存中"
            type="submit"
            variant="primary"
          />
          {isEdit ? (
            <ActionButton
              label="删除服务"
              pending={deletePending}
              pendingLabel="删除中"
              variant="danger"
              onClick={onDelete}
            />
          ) : null}
          <ActionButton label="返回" onClick={onCancel} />
        </div>
      </form>
    </section>
  );
}
