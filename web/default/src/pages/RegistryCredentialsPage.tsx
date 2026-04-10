import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, getErrorMessage, type RegistryCredentialRecord } from "../lib/api";
import { formatDateTime } from "../lib/format";
import styles from "../styles/admin.module.css";

const emptyForm = {
  registryHost: "",
  username: "",
  secret: "",
};

export function RegistryCredentialsPage() {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<RegistryCredentialRecord | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [actionError, setActionError] = useState<string | null>(null);

  const credentialsQuery = useQuery({
    queryKey: ["registry-credentials"],
    queryFn: api.listRegistryCredentials,
  });

  const resetForm = () => {
    setEditing(null);
    setForm(emptyForm);
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (editing) {
        return api.updateRegistryCredential(editing.id, form);
      }
      return api.createRegistryCredential(form);
    },
    onSuccess: async () => {
      setActionError(null);
      resetForm();
      await queryClient.invalidateQueries({ queryKey: ["registry-credentials"] });
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteRegistryCredential(id),
    onSuccess: async (_, id) => {
      setActionError(null);
      if (editing?.id === id) {
        resetForm();
      }
      await queryClient.invalidateQueries({ queryKey: ["registry-credentials"] });
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>镜像仓库凭据</h1>
        </div>
        <div className={styles.buttonRow}>
          <button className={styles.secondaryButton} onClick={() => credentialsQuery.refetch()} type="button">
            刷新
          </button>
          <button className={styles.secondaryButton} onClick={resetForm} type="button">
            新建
          </button>
        </div>
      </section>

      {actionError ? <div className={styles.error}>{actionError}</div> : null}

      <section className={styles.sectionCard}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setActionError(null);
            saveMutation.mutate();
          }}
        >
          <div className={styles.fieldGrid}>
            <label className={styles.field}>
              <span className={styles.label}>Registry Host</span>
              <input
                className={styles.input}
                onChange={(event) => setForm((current) => ({ ...current, registryHost: event.target.value }))}
                value={form.registryHost}
              />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>用户名</span>
              <input
                className={styles.input}
                onChange={(event) => setForm((current) => ({ ...current, username: event.target.value }))}
                value={form.username}
              />
            </label>
            <label className={`${styles.field} ${styles.fieldWide}`}>
              <span className={styles.label}>密码或令牌</span>
              <input
                className={styles.input}
                onChange={(event) => setForm((current) => ({ ...current, secret: event.target.value }))}
                type="password"
                value={form.secret}
              />
            </label>
          </div>

          <div className={styles.buttonRow} style={{ marginTop: 24 }}>
            <button className={styles.primaryButton} disabled={saveMutation.isPending} type="submit">
              {saveMutation.isPending ? "保存中" : editing ? "更新凭据" : "创建凭据"}
            </button>
            {editing ? (
              <button className={styles.secondaryButton} onClick={resetForm} type="button">
                取消编辑
              </button>
            ) : null}
          </div>
        </form>
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.tableWrap}>
          <table>
            <thead>
              <tr>
                <th>Registry Host</th>
                <th>用户名</th>
                <th>已配置密钥</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {credentialsQuery.data?.map((item) => (
                <tr key={item.id}>
                  <td>{item.registryHost}</td>
                  <td>{item.username}</td>
                  <td>{item.secretConfigured ? "是" : "否"}</td>
                  <td>{formatDateTime(item.updatedAt)}</td>
                  <td>
                    <div className={styles.buttonRow}>
                      <button
                        className={styles.secondaryButton}
                        onClick={() => {
                          setEditing(item);
                          setForm({
                            registryHost: item.registryHost,
                            username: item.username,
                            secret: "",
                          });
                        }}
                        type="button"
                      >
                        编辑
                      </button>
                      <button
                        className={styles.secondaryButton}
                        disabled={deleteMutation.isPending}
                        onClick={() => deleteMutation.mutate(item.id)}
                        type="button"
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
