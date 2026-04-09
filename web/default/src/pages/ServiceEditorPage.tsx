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
          <h1 className={styles.sectionTitle}>{isEdit ? "Service Detail" : "New Service"}</h1>
          <p className={styles.sectionCopy}>录入发布必需项，并把复杂结构保持在受控文本块里。</p>
        </div>
        {isEdit && serviceQuery.data ? (
          <StatusPill
            label={serviceQuery.data.enabled ? "Enabled" : "Disabled"}
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
              <span className={styles.label}>Name</span>
              <input className={styles.input} {...form.register("name")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Service Key</span>
              <input className={styles.input} {...form.register("serviceKey")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Agent</span>
              <select className={styles.select} {...form.register("agentId")}>
                <option value="">Select agent</option>
                {agentsQuery.data?.map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.id}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Image Repo</span>
              <input className={styles.input} {...form.register("imageRepo")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Container Port</span>
              <input className={styles.input} type="number" {...form.register("containerPort")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Route Host</span>
              <input className={styles.input} {...form.register("routeHost")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Route Prefix</span>
              <input className={styles.input} {...form.register("routePathPrefix")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>HTTP Health Path</span>
              <input className={styles.input} {...form.register("httpHealthPath")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Expected Code</span>
              <input className={styles.input} type="number" {...form.register("httpExpectedCode")} />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Timeout Seconds</span>
              <input className={styles.input} type="number" {...form.register("httpTimeoutSecond")} />
            </label>
            <label className={styles.field}>
              <span className={styles.checkboxRow}>
                <input type="checkbox" {...form.register("dockerHealthCheck")} />
                <span className={styles.label}>Docker Health Check</span>
              </span>
            </label>
            <label className={styles.field}>
              <span className={styles.checkboxRow}>
                <input type="checkbox" {...form.register("enabled")} />
                <span className={styles.label}>Enabled</span>
              </span>
            </label>

            <label className={`${styles.field} ${styles.fieldWide}`}>
              <span className={styles.label}>Environment</span>
              <textarea className={styles.textarea} {...form.register("envText")} />
              <span className={styles.hint}>One line per entry: `KEY=value`</span>
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Command</span>
              <textarea className={styles.textarea} {...form.register("commandText")} />
              <span className={styles.hint}>One argument per line</span>
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Entrypoint</span>
              <textarea className={styles.textarea} {...form.register("entrypointText")} />
              <span className={styles.hint}>One argument per line</span>
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Volumes</span>
              <textarea className={styles.textarea} {...form.register("volumesText")} />
              <span className={styles.hint}>Format: `/src:/dst[:ro]`</span>
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Published Ports</span>
              <textarea className={styles.textarea} {...form.register("publishedPortsText")} />
              <span className={styles.hint}>Format: `host:container`</span>
            </label>
          </div>

          {submitError ? <div className={styles.error}>{submitError}</div> : null}

          <div className={styles.buttonRow} style={{ marginTop: 24 }}>
            <button className={styles.primaryButton} disabled={saveMutation.isPending} type="submit">
              {saveMutation.isPending ? "Saving" : isEdit ? "Update Service" : "Create Service"}
            </button>
            <button className={styles.secondaryButton} onClick={() => navigate("/services")} type="button">
              Back
            </button>
          </div>
        </form>
      </section>

      {isEdit && serviceQuery.data ? (
        <>
          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className={styles.sectionTitle}>Runtime Summary</h2>
                <p className={styles.sectionCopy}>已落库的运行实例与流量状态。</p>
              </div>
            </div>
            <div className={styles.keyValueGrid}>
              <div className={styles.keyValue}>
                <span className={styles.key}>Live Slot</span>
                <span className={styles.value}>{slotLabel(serviceQuery.data.currentLiveSlot)}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>Docker Health</span>
                <span className={styles.value}>{boolLabel(serviceQuery.data.dockerHealthCheck)}</span>
              </div>
              <div className={styles.keyValue}>
                <span className={styles.key}>Updated</span>
                <span className={styles.value}>{formatDateTime(serviceQuery.data.updatedAt)}</span>
              </div>
            </div>
          </section>

          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className={styles.sectionTitle}>Observability</h2>
                <p className={styles.sectionCopy}>这里轮询运行实例和 backend snapshot。</p>
              </div>
            </div>
            <div className={styles.tableWrap}>
              <table>
                <thead>
                  <tr>
                    <th>Server</th>
                    <th>Slot</th>
                    <th>Image</th>
                    <th>Healthy</th>
                    <th>Traffic</th>
                    <th>Updated</th>
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
                    <th>Backend</th>
                    <th>Server</th>
                    <th>SCur</th>
                    <th>Rate</th>
                    <th>Error Requests</th>
                    <th>Created</th>
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
