import { formatDateTime, boolLabel } from "../../../shared/lib/format";
import { StatusPill } from "../../../shared/components/StatusPill";
import type { AgentRecord } from "../types";
import type { ServiceRecord } from "../../services/types";
import styles from "../../../styles/admin.module.css";

interface AgentInfoProps {
  agent: AgentRecord;
  services: ServiceRecord[];
  servicesPending: boolean;
  servicesError: boolean;
}

export function AgentInfo({ agent, services, servicesPending, servicesError }: AgentInfoProps) {
  const preboundPorts = [...new Set(agent.preboundTcpPorts ?? [])].sort((a, b) => a - b);
  const preboundSet = new Set(preboundPorts);
  const servicesByPort = new Map<number, string[]>();
  for (const service of services) {
    if (service.agentId !== agent.id) {
      continue;
    }
    for (const mapping of service.tcpProxyPorts ?? []) {
      if (!preboundSet.has(mapping.listenPort)) {
        continue;
      }
      const names = servicesByPort.get(mapping.listenPort) ?? [];
      if (!names.includes(service.name)) {
        names.push(service.name);
      }
      servicesByPort.set(mapping.listenPort, names);
    }
  }
  const availablePorts = preboundPorts.filter((port) => !servicesByPort.has(port));
  const occupiedPorts = [...servicesByPort.entries()].sort(([left], [right]) => left - right);

  return (
    <section className={styles.sectionCard}>
      <div className={styles.keyValueGrid}>
        <div className={styles.keyValue}>
          <span className={styles.key}>节点 ID</span>
          <span className={styles.value}>{agent.id}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>在线状态</span>
          <span className={styles.value}>{boolLabel(agent.online, "在线", "离线")}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>启用状态</span>
          <span className={styles.value}>{boolLabel(agent.enabled, "启用", "停用")}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>主机名</span>
          <span className={styles.value}>{agent.hostname || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>IP</span>
          <span className={styles.value}>{agent.ip || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>版本</span>
          <span className={styles.value}>{agent.version || "—"}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>最近连接</span>
          <span className={styles.value}>{formatDateTime(agent.lastConnectedAt)}</span>
        </div>
        <div className={styles.keyValue}>
          <span className={styles.key}>最近心跳</span>
          <span className={styles.value}>{formatDateTime(agent.lastHeartbeatAt)}</span>
        </div>
      </div>
      <div className={styles.tcpPrebindSection}>
        <div className={styles.tcpPrebindHeader}>
          <div>
            <h2 className={styles.tcpPrebindTitle}>TCP 预占端口</h2>
            <p className={styles.tcpPrebindMeta}>
              {agent.tcpPrebindSupported ? `实际预占 ${preboundPorts.length} / 100` : "当前版本未上报预占端口"}
            </p>
          </div>
          {!agent.online && agent.tcpPrebindSupported ? (
            <span className={styles.tcpPrebindStale}>最近一次连接时上报</span>
          ) : null}
        </div>
        {agent.tcpPrebindSupported ? (
          servicesError ? (
            <p className={styles.tcpPrebindError}>服务数据加载失败，暂时无法计算可用端口。</p>
          ) : servicesPending ? (
            <p className={styles.tcpPrebindEmpty}>正在计算端口占用状态…</p>
          ) : (
            <div className={styles.tcpPrebindGroups}>
              <div className={styles.tcpPrebindGroup}>
                <span className={styles.tcpPrebindLabel}>当前可用 · {availablePorts.length}</span>
                {availablePorts.length > 0 ? (
                  <div className={styles.tcpPortList}>
                    {availablePorts.map((port) => (
                      <code className={styles.tcpPortToken} key={port}>
                        {port}
                      </code>
                    ))}
                  </div>
                ) : (
                  <p className={styles.tcpPrebindEmpty}>暂无可用预占端口</p>
                )}
              </div>
              <div className={styles.tcpPrebindGroup}>
                <span className={styles.tcpPrebindLabel}>已占用 · {occupiedPorts.length}</span>
                {occupiedPorts.length > 0 ? (
                  <div className={styles.tcpPortList}>
                    {occupiedPorts.map(([port, names]) => (
                      <span className={styles.tcpPortAssignment} key={port}>
                        <code>{port}</code>
                        <span>{names.join("、")}</span>
                      </span>
                    ))}
                  </div>
                ) : (
                  <p className={styles.tcpPrebindEmpty}>暂无服务占用预占端口</p>
                )}
              </div>
            </div>
          )
        ) : null}
      </div>
      <StatusPill label={agent.lastError || "最近没有错误"} tone={agent.lastError ? "danger" : "success"} />
    </section>
  );
}
