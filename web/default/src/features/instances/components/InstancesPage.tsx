import { Fragment, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { instancesApi } from "../api";
import type { ManagedInstanceRecord } from "../types";
import { ContainerLogDialog } from "./ContainerLogDialog";
import styles from "../../../styles/admin.module.css";

export function InstancesPage() {
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [logInstance, setLogInstance] = useState<ManagedInstanceRecord | null>(null);
  const [stateFilter, setStateFilter] = useState<string>("");
  const [searchQuery, setSearchQuery] = useState("");

  const instancesQuery = useQuery({
    queryKey: ["instances"],
    queryFn: instancesApi.list,
    refetchInterval: 10000,
  });

  const filteredInstances = (instancesQuery.data ?? []).filter((instance) => {
    if (stateFilter && instance.state !== stateFilter) return false;
    if (searchQuery && !instance.name.toLowerCase().includes(searchQuery.toLowerCase())) return false;
    return true;
  });

  const stateOptions = Array.from(new Set((instancesQuery.data ?? []).map((i) => i.state)));

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>受管实例</h1>
          <p className={styles.sectionCopy}>查看所有节点上受管容器的状态、元数据与实时日志。</p>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton label="刷新" onClick={() => instancesQuery.refetch()} />
        </div>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.filterRow}>
          <select
            className={styles.select}
            value={stateFilter}
            onChange={(e) => setStateFilter(e.target.value)}
          >
            <option value="">全部状态</option>
            {stateOptions.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
          <input
            type="text"
            className={styles.input}
            placeholder="搜索容器名称..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>

        {instancesQuery.isPending ? (
          <LoadingState title="正在加载实例列表" message="正在同步各节点容器状态。" />
        ) : instancesQuery.isError ? (
          <ErrorState
            title="实例列表加载失败"
            message={getErrorMessage(instancesQuery.error)}
            onRetry={() => instancesQuery.refetch()}
          />
        ) : !filteredInstances.length ? (
          <EmptyState title="暂无实例" message="当前没有可查看的受管容器。" />
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <thead>
                <tr>
                  <th>容器名称</th>
                  <th>状态</th>
                  <th>节点</th>
                  <th>服务</th>
                  <th>Slot</th>
                  <th>镜像</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {filteredInstances.map((instance) => (
                  <Fragment key={instance.containerId}>
                    <tr
                      className={styles.clickableRow}
                      onClick={() =>
                        setExpandedId(expandedId === instance.containerId ? null : instance.containerId)
                      }
                    >
                      <td>{instance.name}</td>
                      <td>
                        <StatusPill
                          label={instance.state}
                          tone={
                            instance.state === "running"
                              ? "success"
                              : instance.state === "exited"
                                ? "danger"
                                : "warning"
                          }
                        />
                      </td>
                      <td>{instance.agentHost || instance.agentId}</td>
                      <td>{instance.serviceKey || "—"}</td>
                      <td>{instance.slot || "—"}</td>
                      <td>{instance.image || "—"}</td>
                      <td>{instance.createdAt ? formatDateTime(new Date(instance.createdAt * 1000).toISOString()) : "—"}</td>
                      <td>
                        <button
                          className={styles.linkButton}
                          onClick={(e) => {
                            e.stopPropagation();
                            setLogInstance(instance);
                          }}
                        >
                          查看日志
                        </button>
                      </td>
                    </tr>
                    {expandedId === instance.containerId && (
                      <tr>
                        <td colSpan={8}>
                          <InstanceDetailsPanel agentId={instance.agentId} containerId={instance.containerId} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {logInstance && (
        <ContainerLogDialog
          agentId={logInstance.agentId}
          containerId={logInstance.containerId}
          containerName={logInstance.name}
          onClose={() => setLogInstance(null)}
        />
      )}
    </div>
  );
}

function InstanceDetailsPanel({ agentId, containerId }: { agentId: string; containerId: string }) {
  const detailsQuery = useQuery({
    queryKey: ["instance", agentId, containerId],
    queryFn: () => instancesApi.get(agentId, containerId),
    enabled: Boolean(agentId && containerId),
  });

  if (detailsQuery.isPending) {
    return <div className={styles.detailPanel}>正在加载详情...</div>;
  }

  if (detailsQuery.isError) {
    return <div className={styles.detailPanel}>详情加载失败: {getErrorMessage(detailsQuery.error)}</div>;
  }

  const d = detailsQuery.data;
  if (!d) return null;

  return (
    <div className={styles.detailPanel}>
      <div className={styles.detailGrid}>
        <div className={styles.detailGroup}>
          <h4>基础信息</h4>
          <div><strong>容器 ID:</strong> {d.containerId}</div>
          <div><strong>状态:</strong> {d.state} {d.running ? "(运行中)" : ""}</div>
          <div><strong>健康检查:</strong> {d.health || "—"}</div>
          <div><strong>重启次数:</strong> {d.restartCount}</div>
        </div>
        <div className={styles.detailGroup}>
          <h4>归属信息</h4>
          <div><strong>Agent:</strong> {d.agentHost || d.agentId}</div>
          <div><strong>服务:</strong> {d.serviceKey || "—"}</div>
          <div><strong>Release:</strong> {d.releaseId || "—"}</div>
          <div><strong>Slot:</strong> {d.slot || "—"}</div>
        </div>
        <div className={styles.detailGroup}>
          <h4>网络</h4>
          <div><strong>内部 IP:</strong> {d.ipAddress || "—"}</div>
          <div><strong>镜像:</strong> {d.image || "—"}</div>
        </div>
        <div className={styles.detailGroup}>
          <h4>资源</h4>
          <div><strong>CPU 限制:</strong> {d.cpuLimit > 0 ? `${d.cpuLimit} 核` : "—"}</div>
          <div><strong>内存限制:</strong> {d.memoryLimit > 0 ? `${d.memoryLimit} MB` : "—"}</div>
        </div>
      </div>
      {d.env && Object.keys(d.env).length > 0 && (
        <div className={styles.detailGroup}>
          <h4>环境变量</h4>
          {Object.entries(d.env).map(([k, v]) => (
            <div key={k}><code>{k}={v}</code></div>
          ))}
        </div>
      )}
    </div>
  );
}
