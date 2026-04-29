import { useQuery } from "@tanstack/react-query";
import { formatDateTime, slotLabel, boolLabel } from "../../../shared/lib/format";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { servicesApi } from "../api";
import styles from "../../../styles/admin.module.css";

interface ServiceObservabilityProps {
  serviceId: string;
}

export function ServiceObservability({ serviceId }: ServiceObservabilityProps) {
  const observabilityQuery = useQuery({
    queryKey: ["service-observability", serviceId],
    queryFn: () => servicesApi.getObservability(serviceId),
    refetchInterval: 10000,
  });

  return (
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
  );
}
