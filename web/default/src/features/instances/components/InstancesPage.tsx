import { Fragment, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { StatusPill } from "../../../shared/components/StatusPill";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { instancesApi } from "../api";
import type { ManagedInstanceRecord } from "../types";
import { ContainerLogDialog } from "./ContainerLogDialog";
import styles from "../../../styles/admin.module.css";

export function InstancesPage() {
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [logInstance, setLogInstance] = useState<ManagedInstanceRecord | null>(
    null,
  );
  const [stateFilter, setStateFilter] = useState<string>("");
  const [searchQuery, setSearchQuery] = useState("");

  const instancesQuery = useQuery({
    queryKey: ["instances"],
    queryFn: instancesApi.list,
    refetchInterval: 10000,
  });

  const filteredInstances = (instancesQuery.data ?? []).filter((instance) => {
    if (stateFilter && instance.state !== stateFilter) return false;
    if (
      searchQuery &&
      ![
        instance.name,
        instance.containerId,
        instance.agentHost,
        instance.agentId,
        instance.serviceKey,
        instance.image,
      ].some((value) =>
        value.toLowerCase().includes(searchQuery.trim().toLowerCase()),
      )
    )
      return false;
    return true;
  });

  const stateOptions = Array.from(
    new Set((instancesQuery.data ?? []).map((i) => i.state)),
  );

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>受管实例</h1>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新"
            pending={instancesQuery.isFetching}
            onClick={() => void instancesQuery.refetch()}
          />
        </div>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.toolbar}>
          <label className={styles.filterField}>
            <span className={styles.visuallyHidden}>实例状态</span>
            <select
              aria-label="实例状态"
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
          </label>
          <label className={styles.searchField}>
            <span className={styles.visuallyHidden}>搜索实例</span>
            <input
              type="search"
              aria-label="搜索实例"
              className={styles.input}
              placeholder="搜索容器、服务、镜像或节点"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </label>
          <span className={styles.resultCount}>
            {filteredInstances.length} / {instancesQuery.data?.length ?? 0}{" "}
            个实例
          </span>
        </div>
        {instancesQuery.data ? (
          <QueryRefreshNotice
            query={instancesQuery}
            onRetry={() => void instancesQuery.refetch()}
          />
        ) : null}

        {instancesQuery.isPending ? (
          <LoadingState
            title="正在加载实例列表"
            message="正在同步各节点容器状态。"
          />
        ) : instancesQuery.isError && !instancesQuery.data ? (
          <ErrorState
            title="实例列表加载失败"
            message={getErrorMessage(instancesQuery.error)}
            onRetry={() => instancesQuery.refetch()}
          />
        ) : !filteredInstances.length ? (
          <EmptyState
            title={instancesQuery.data?.length ? "没有匹配的实例" : "暂无实例"}
            message={
              instancesQuery.data?.length
                ? "尝试更改搜索或状态筛选。"
                : "当前没有受管容器。"
            }
            actionLabel={searchQuery || stateFilter ? "清除筛选" : undefined}
            onRetry={() => {
              setSearchQuery("");
              setStateFilter("");
            }}
          />
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
                    <tr>
                      <td>
                        <button
                          type="button"
                          className={styles.tableLink}
                          aria-expanded={expandedId === instance.containerId}
                          aria-controls={
                            expandedId === instance.containerId
                              ? `instance-${instance.containerId}`
                              : undefined
                          }
                          onClick={() =>
                            setExpandedId(
                              expandedId === instance.containerId
                                ? null
                                : instance.containerId,
                            )
                          }
                        >
                          {instance.name}
                        </button>
                        <div className={styles.hint}>
                          {instance.containerId.slice(0, 12)}
                        </div>
                      </td>
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
                      <td className={styles.mono}>{instance.image || "—"}</td>
                      <td>
                        {instance.createdAt
                          ? formatDateTime(
                              new Date(instance.createdAt * 1000).toISOString(),
                            )
                          : "—"}
                      </td>
                      <td>
                        <button
                          type="button"
                          className={styles.ghostButton}
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
                        <td colSpan={8} id={`instance-${instance.containerId}`}>
                          <InstanceDetailsPanel
                            agentId={instance.agentId}
                            containerId={instance.containerId}
                          />
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

function InstanceDetailsPanel({
  agentId,
  containerId,
}: {
  agentId: string;
  containerId: string;
}) {
  const detailsQuery = useQuery({
    queryKey: ["instance", agentId, containerId],
    queryFn: () => instancesApi.get(agentId, containerId),
    enabled: Boolean(agentId && containerId),
  });

  if (detailsQuery.isPending) {
    return <LoadingState title="正在加载实例详情" />;
  }

  if (detailsQuery.isError && !detailsQuery.data) {
    return (
      <ErrorState
        title="实例详情加载失败"
        message={getErrorMessage(detailsQuery.error)}
        onRetry={() => void detailsQuery.refetch()}
      />
    );
  }

  const d = detailsQuery.data;
  if (!d) return null;

  return (
    <div className={styles.detailPanel}>
      <QueryRefreshNotice
        query={detailsQuery}
        onRetry={() => void detailsQuery.refetch()}
      />
      <div className={styles.detailGrid}>
        <div className={styles.detailGroup}>
          <h4>基础信息</h4>
          <div>
            <strong>容器 ID:</strong> {d.containerId}
          </div>
          <div>
            <strong>状态:</strong> {d.state} {d.running ? "(运行中)" : ""}
          </div>
          <div>
            <strong>健康检查:</strong> {d.health || "—"}
          </div>
          <div>
            <strong>重启次数:</strong> {d.restartCount}
          </div>
        </div>
        <div className={styles.detailGroup}>
          <h4>归属信息</h4>
          <div>
            <strong>Agent:</strong> {d.agentHost || d.agentId}
          </div>
          <div>
            <strong>服务:</strong> {d.serviceKey || "—"}
          </div>
          <div>
            <strong>Release:</strong> {d.releaseId || "—"}
          </div>
          <div>
            <strong>Slot:</strong> {d.slot || "—"}
          </div>
        </div>
        <div className={styles.detailGroup}>
          <h4>网络</h4>
          <div>
            <strong>内部 IP:</strong> {d.ipAddress || "—"}
          </div>
          <div>
            <strong>镜像:</strong> {d.image || "—"}
          </div>
        </div>
        <div className={styles.detailGroup}>
          <h4>资源</h4>
          <div>
            <strong>CPU 限制:</strong>{" "}
            {d.cpuLimit > 0 ? `${d.cpuLimit} 核` : "—"}
          </div>
          <div>
            <strong>内存限制:</strong>{" "}
            {d.memoryLimit > 0 ? `${d.memoryLimit} MB` : "—"}
          </div>
        </div>
      </div>
      {d.env && Object.keys(d.env).length > 0 && (
        <div className={styles.detailGroup}>
          <h4>环境变量</h4>
          {Object.entries(d.env).map(([k, v]) => (
            <div key={k}>
              <code>
                {k}={v}
              </code>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
