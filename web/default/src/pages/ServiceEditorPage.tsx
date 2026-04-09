import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useNavigate, useParams } from "react-router-dom";
import { api, getErrorMessage } from "../lib/api";
import { formatDateTime, slotLabel, boolLabel } from "../lib/format";
import {
  serviceFormSchema,
  toServiceFormDefaults,
  toServicePayload,
  type ServiceFormInput,
  type ServiceFormValues,
} from "../lib/forms";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function ServiceEditorPage() {
  const params = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const serviceId = params.id;
  const isEdit = Boolean(serviceId);

  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: api.listAgents,
  });
  const serviceQuery = useQuery({
    queryKey: ["service", serviceId],
    queryFn: () => api.getService(serviceId!),
    enabled: isEdit,
  });
  const observabilityQuery = useQuery({
    queryKey: ["service-observability", serviceId],
    queryFn: () => api.getServiceObservability(serviceId!),
    enabled: isEdit,
    refetchInterval: 10000,
  });

  const form = useForm<ServiceFormInput, unknown, ServiceFormValues>({
    resolver: zodResolver(serviceFormSchema),
    defaultValues: toServiceFormDefaults(),
  });

  useEffect(() => {
    if (serviceQuery.data) {
      form.reset(toServiceFormDefaults(serviceQuery.data));
    }
  }, [form, serviceQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async (values: ServiceFormValues) => {
      const payload = toServicePayload(values);
      return isEdit ? api.updateService(serviceId!, payload) : api.createService(payload);
    },
    onSuccess: async (service) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["services"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
      navigate(`/services/${service.id}`, { replace: true });
    },
    onError: (error) => setSubmitError(getErrorMessage(error)),
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>{isEdit ? "服务详情" : "新建服务"}</h1>
          <p className={styles.sectionCopy}>录入发布必需项，并把复杂结构保持在受控文本块里。</p>
        </div>
        {isEdit && serviceQuery.data ? (
          <StatusPill
            label={serviceQuery.data.enabled ? "启用" : "停用"}
            tone={serviceQuery.data.enabled ? "success" : "danger"}
          />
        ) : null}
      </section>

      <section className={styles.sectionCard}>
        <form
          onSubmit={form.handleSubmit((values) => {
            setSubmitError(null);
            saveMutation.mutate(values);
          })}
        >
          <div className={styles.fieldGrid}>
            <label className={styles.field}>
              <span className={styles.label}>名称</span>
              <input className={styles.input} {...form.register("name")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>服务标识</span>
              <input className={styles.input} {...form.register("serviceKey")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>节点</span>
              <select className={styles.select} {...form.register("agentId")}>
                <option value="">选择节点</option>
                {agentsQuery.data?.map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.id}
                  </option>
                ))}
              </select>
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
              <span className={styles.label}>路由 Host</span>
              <input className={styles.input} {...form.register("routeHost")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>路由前缀</span>
              <input className={styles.input} {...form.register("routePathPrefix")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>HTTP 探活路径</span>
              <input className={styles.input} {...form.register("httpHealthPath")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>预期状态码</span>
              <input className={styles.input} type="number" {...form.register("httpExpectedCode")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>超时秒数</span>
              <input className={styles.input} type="number" {...form.register("httpTimeoutSecond")} />
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
              <span className={styles.label}>暴露端口</span>
              <textarea className={styles.textarea} {...form.register("publishedPortsText")} />
              <span className={styles.hint}>格式：`host:container`</span>
            </label>
          </div>

          {submitError ? <div className={styles.error}>{submitError}</div> : null}

          <div className={styles.buttonRow} style={{ marginTop: 24 }}>
            <button className={styles.primaryButton} disabled={saveMutation.isPending} type="submit">
              {saveMutation.isPending ? "保存中" : isEdit ? "更新服务" : "创建服务"}
            </button>
            <button className={styles.secondaryButton} onClick={() => navigate("/services")} type="button">
              返回
            </button>
          </div>
        </form>
      </section>

      {isEdit && serviceQuery.data ? (
        <>
          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className={styles.sectionTitle}>运行摘要</h2>
                <p className={styles.sectionCopy}>已落库的运行实例与流量状态。</p>
              </div>
            </div>
            <div className={styles.keyValueGrid}>
              <div className={styles.keyValue}>
                <span className={styles.key}>当前槽位</span>
                <span className={styles.value}>{slotLabel(serviceQuery.data.currentLiveSlot)}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>Docker 探活</span>
                <span className={styles.value}>{boolLabel(serviceQuery.data.dockerHealthCheck)}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>更新时间</span>
                <span className={styles.value}>{formatDateTime(serviceQuery.data.updatedAt)}</span>
              </div>
            </div>
          </section>

          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className={styles.sectionTitle}>运行观测</h2>
                <p className={styles.sectionCopy}>这里轮询运行实例和 backend snapshot。</p>
              </div>
            </div>
            <div className={styles.tableWrap}>
              <table>
                <thead>
                  <tr>
                    <th>服务端点</th>
                    <th>槽位</th>
                    <th>镜像</th>
                    <th>健康</th>
                    <th>接流</th>
                    <th>更新时间</th>
                  </tr>
                </thead>
                <tbody>
                  {observabilityQuery.data?.runtimeInstances.map((item) => (
                    <tr key={item.id}>
                      <td>{item.serverName}</td>
                      <td>{slotLabel(item.slot)}</td>
                      <td>{item.imageTag}</td>
                      <td>{boolLabel(item.healthy)}</td>
                      <td>{boolLabel(item.acceptingTraffic)}</td>
                      <td>{formatDateTime(item.updatedAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className={styles.tableWrap}>
              <table>
                <thead>
                  <tr>
                    <th>后端</th>
                    <th>服务端点</th>
                    <th>SCur</th>
                    <th>Rate</th>
                    <th>错误请求</th>
                    <th>采集时间</th>
                  </tr>
                </thead>
                <tbody>
                  {observabilityQuery.data?.backendStats.map((item) => (
                    <tr key={item.backendName + item.serverName + item.createdAt}>
                      <td>{item.backendName}</td>
                      <td>{item.serverName}</td>
                      <td>{item.scur}</td>
                      <td>{item.rate}</td>
                      <td>{item.errorRequests}</td>
                      <td>{formatDateTime(item.createdAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        </>
      ) : null}
    </div>
  );
}
