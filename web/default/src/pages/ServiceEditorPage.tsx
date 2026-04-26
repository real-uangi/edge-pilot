import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useNavigate, useParams } from "react-router-dom";
import { api, getErrorMessage } from "../lib/api";
import { formatAgentLabel, formatDateTime, slotLabel, boolLabel } from "../lib/format";
import {
  serviceFormSchema,
  toServiceFormDefaults,
  toServicePayload,
  type ServiceFormInput,
  type ServiceFormValues,
} from "../lib/forms";
import { ActionButton } from "../components/ActionButton";
import { StatusPill } from "../components/StatusPill";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../components/StateBlocks";
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
      if (isEdit && serviceQuery.data) {
        payload.serviceKey = serviceQuery.data.serviceKey;
        payload.agentId = serviceQuery.data.agentId;
      }
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

  if (isEdit && serviceQuery.isPending) {
    return (
      <div className={styles.page}>
        <LoadingState title="正在加载服务详情" message="正在拉取服务配置与运行摘要。" />
      </div>
    );
  }

  if (isEdit && serviceQuery.isError) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="服务详情加载失败"
          message={getErrorMessage(serviceQuery.error)}
          onRetry={() => serviceQuery.refetch()}
        />
      </div>
    );
  }

  if (isEdit && !serviceQuery.data) {
    return (
      <div className={styles.page}>
        <EmptyState title="服务不存在" message="请返回服务列表后重新选择。" />
      </div>
    );
  }

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>{isEdit ? "服务详情" : "新建服务"}</h1>
          <p className={styles.sectionCopy}>{isEdit ? "更新服务配置并同步运行策略。" : "填写服务基础信息并创建配置。"}</p>
        </div>
        {isEdit && serviceQuery.data ? (
          <StatusPill
            label={serviceQuery.data.enabled ? "启用" : "停用"}
            tone={serviceQuery.data.enabled ? "success" : "danger"}
          />
        ) : null}
      </section>

      {agentsQuery.isError ? <InlineNotice message={`节点列表加载失败：${getErrorMessage(agentsQuery.error)}`} tone="error" /> : null}

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
              pending={saveMutation.isPending}
              pendingLabel="保存中"
              type="submit"
              variant="primary"
            />
            <ActionButton label="返回" onClick={() => navigate("/services")} />
          </div>
        </form>
      </section>

      {isEdit && serviceQuery.data ? (
        <>
          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className={styles.sectionTitle}>运行摘要</h2>
              </div>
            </div>
            <div className={styles.keyValueGrid}>
              <div className={styles.keyValue}>
                <span className={styles.key}>当前槽位</span>
                <span className={styles.value}>{slotLabel(serviceQuery.data.currentLiveSlot)}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>调度端口</span>
                <span className={styles.value}>{serviceQuery.data.schedulerSdkPort || "-"}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>执行器组</span>
                <span className={styles.value}>{serviceQuery.data.schedulerExecutorGroup || "-"}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>Docker 探活</span>
                <span className={styles.value}>{boolLabel(serviceQuery.data.dockerHealthCheck)}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>预热窗口</span>
                <span className={styles.value}>{serviceQuery.data.httpTimeoutSecond}s</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>启动宽限</span>
                <span className={styles.value}>{serviceQuery.data.startupGraceSecond}s</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>探测节奏</span>
                <span className={styles.value}>
                  {serviceQuery.data.httpProbeIntervalSecond}s / timeout {serviceQuery.data.httpProbeTimeoutSecond}s
                </span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>连续成功</span>
                <span className={styles.value}>{serviceQuery.data.httpSuccessThreshold} 次</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>探活 Header</span>
                <span className={styles.value}>{Object.keys(serviceQuery.data.httpHealthHeaders ?? {}).length} 项</span>
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
              </div>
            </div>

            {observabilityQuery.isPending ? (
              <LoadingState title="正在加载运行观测" message="正在拉取实例与后端统计。" />
            ) : observabilityQuery.isError ? (
              <ErrorState
                title="运行观测加载失败"
                message={getErrorMessage(observabilityQuery.error)}
                onRetry={() => observabilityQuery.refetch()}
              />
            ) : (
              <>
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
                      {observabilityQuery.data?.runtimeInstances.length ? (
                        observabilityQuery.data.runtimeInstances.map((item) => (
                          <tr key={item.id}>
                            <td>{item.serverName}</td>
                            <td>{slotLabel(item.slot)}</td>
                            <td>{item.imageTag}</td>
                            <td>{boolLabel(item.healthy)}</td>
                            <td>{boolLabel(item.acceptingTraffic)}</td>
                            <td>{formatDateTime(item.updatedAt)}</td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={6}>暂无实例观测数据</td>
                        </tr>
                      )}
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
                      {observabilityQuery.data?.backendStats.length ? (
                        observabilityQuery.data.backendStats.map((item) => (
                          <tr key={item.backendName + item.serverName + item.createdAt}>
                            <td>{item.backendName}</td>
                            <td>{item.serverName}</td>
                            <td>{item.scur}</td>
                            <td>{item.rate}</td>
                            <td>{item.errorRequests}</td>
                            <td>{formatDateTime(item.createdAt)}</td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={6}>暂无后端统计数据</td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </section>
        </>
      ) : null}
    </div>
  );
}
