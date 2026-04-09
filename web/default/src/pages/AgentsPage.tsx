import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, getErrorMessage, type AgentCredentialRecord } from "../lib/api";
import { formatDateTime, boolLabel } from "../lib/format";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function AgentsPage() {
  const queryClient = useQueryClient();
  const [credential, setCredential] = useState<AgentCredentialRecord | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: api.listAgents,
    refetchInterval: 10000,
  });

  const createMutation = useMutation({
    mutationFn: api.createAgent,
    onSuccess: async (output) => {
      setCredential(output);
      setActionError(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["agents"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>Agents</h1>
          <p className={styles.sectionCopy}>管理注册节点、在线态和一次性 token 派发。</p>
        </div>
        <div className={styles.buttonRow}>
          <button className={styles.secondaryButton} onClick={() => agentsQuery.refetch()} type="button">
            Refresh
          </button>
          <button className={styles.primaryButton} disabled={createMutation.isPending} onClick={() => createMutation.mutate()} type="button">
            {createMutation.isPending ? "Creating" : "Create Agent"}
          </button>
        </div>
      </section>

      {actionError ? <div className={styles.error}>{actionError}</div> : null}

      {credential ? (
        <section className={styles.credentialCard}>
          <span className={styles.eyebrow}>Issued Credential</span>
          <div>
            <strong>ID</strong>
            <div className={styles.credentialValue}>{credential.id}</div>
          </div>
          <div>
            <strong>Token</strong>
            <div className={styles.credentialValue}>{credential.token}</div>
          </div>
        </section>
      ) : null}

      <section className={styles.sectionCard}>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>Agent</th>
                <th>Hostname</th>
                <th>Version</th>
                <th>Online</th>
                <th>Enabled</th>
                <th>Heartbeat</th>
              </tr>
            </thead>
            <tbody>
              {agentsQuery.data?.map((agent) => (
                <tr key={agent.id}>
                  <td>
                    <Link className={styles.tableLink} to={`/agents/${agent.id}`}>
                      {agent.id}
                    </Link>
                  </td>
                  <td>{agent.hostname || "—"}</td>
                  <td>{agent.version || "—"}</td>
                  <td>
                    <StatusPill
                      label={boolLabel(agent.online, "Online", "Offline")}
                      tone={agent.online ? "success" : "danger"}
                    />
                  </td>
                  <td>{boolLabel(agent.enabled, "Enabled", "Disabled")}</td>
                  <td>{formatDateTime(agent.lastHeartbeatAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
