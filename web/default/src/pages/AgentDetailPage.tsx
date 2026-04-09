import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { api, getErrorMessage, type AgentCredentialRecord } from "../lib/api";
import { formatDateTime, boolLabel } from "../lib/format";
import { StatusPill } from "../components/StatusPill";
import styles from "../styles/admin.module.css";

export function AgentDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [issuedCredential, setIssuedCredential] = useState<AgentCredentialRecord | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const agentQuery = useQuery({
    queryKey: ["agent", id],
    queryFn: () => api.getAgent(id!),
    enabled: Boolean(id),
    refetchInterval: 10000,
  });

  const refreshQueries = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["agent", id] }),
      queryClient.invalidateQueries({ queryKey: ["agents"] }),
      queryClient.invalidateQueries({ queryKey: ["overview"] }),
    ]);
  };

  const enableMutation = useMutation({
    mutationFn: () => api.enableAgent(id!),
    onSuccess: async () => {
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const disableMutation = useMutation({
    mutationFn: () => api.disableAgent(id!),
    onSuccess: async () => {
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });
  const resetMutation = useMutation({
    mutationFn: () => api.resetAgentToken(id!),
    onSuccess: async (output) => {
      setIssuedCredential(output);
      setActionError(null);
      await refreshQueries();
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });

  if (agentQuery.isPending) {
    return <div className={styles.page}>Loading agent…</div>;
  }
  if (!agentQuery.data) {
    return <div className={styles.page}>Agent not found.</div>;
  }

  const agent = agentQuery.data;

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>Agent Detail</h1>
          <p className={styles.sectionCopy}>{agent.id}</p>
        </div>
        <div className={styles.buttonRow}>
          <button className={styles.secondaryButton} onClick={() => navigate("/agents")} type="button">
            Back
          </button>
          <button className={styles.secondaryButton} onClick={() => agentQuery.refetch()} type="button">
            Refresh
          </button>
          <button
            className={styles.ghostButton}
            disabled={enableMutation.isPending}
            onClick={() => enableMutation.mutate()}
            type="button"
          >
            Enable
          </button>
          <button
            className={styles.dangerButton}
            disabled={disableMutation.isPending}
            onClick={() => disableMutation.mutate()}
            type="button"
          >
            Disable
          </button>
          <button
            className={styles.primaryButton}
            disabled={resetMutation.isPending}
            onClick={() => resetMutation.mutate()}
            type="button"
          >
            {resetMutation.isPending ? "Resetting" : "Reset Token"}
          </button>
        </div>
      </section>

      {actionError ? <div className={styles.error}>{actionError}</div> : null}

      {issuedCredential ? (
        <section className={styles.credentialCard}>
          <span className={styles.eyebrow}>One Time Token</span>
          <div className={styles.credentialValue}>{issuedCredential.token}</div>
        </section>
      ) : null}

      <section className={styles.sectionCard}>
        <div className={styles.keyValueGrid}>
          <div className={styles.keyValue}>
            <span className={styles.key}>Online</span>
            <span className={styles.value}>{boolLabel(agent.online, "Online", "Offline")}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Enabled</span>
            <span className={styles.value}>{boolLabel(agent.enabled, "Enabled", "Disabled")}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Hostname</span>
            <span className={styles.value}>{agent.hostname || "—"}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Version</span>
            <span className={styles.value}>{agent.version || "—"}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Connected</span>
            <span className={styles.value}>{formatDateTime(agent.lastConnectedAt)}</span>
          </div>
          <div className={styles.keyValue}>
            <span className={styles.key}>Heartbeat</span>
            <span className={styles.value}>{formatDateTime(agent.lastHeartbeatAt)}</span>
          </div>
        </div>
        <StatusPill label={agent.lastError || "No recent error"} tone={agent.lastError ? "danger" : "success"} />
      </section>
    </div>
  );
}
