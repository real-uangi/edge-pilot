import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import { EmptyState, ErrorState, InlineNotice, LoadingState } from "../../../shared/components/StateBlocks";
import { registryCredentialsApi } from "../api";
import type { RegistryCredentialRecord } from "../types";
import styles from "../../../styles/admin.module.css";

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
    queryFn: registryCredentialsApi.list,
  });

  const resetForm = () => {
    setEditing(null);
    setForm(emptyForm);
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (editing) {
        return registryCredentialsApi.update(editing.id, form);
      }
      return registryCredentialsApi.create(form);
    },
    onSuccess: async () => {
      setActionError(null);
      resetForm();
      await queryClient.invalidateQueries({ queryKey: ["registry-credentials"] });
    },
    onError: (error) => setActionError(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => registryCredentialsApi.delete(id),
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
          <p className={styles.sectionCopy}>维护镜像仓库认证信息，支持新增、更新和删除。</p>
        </div>
        <div className={styles.buttonRow}>
          <ActionButton label="刷新" onClick={() => credentialsQuery.refetch()} />
          <ActionButton label="新建" onClick={resetForm} />
        </div>
      </section>

      {actionError ? <InlineNotice message={actionError} tone="error" /> : null}

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
            <ActionButton
              label={editing ? "更新凭据" : "创建凭据"}
              pending={saveMutation.isPending}
              pendingLabel="保存中"
              type="submit"
              variant="primary"
            />
            {editing ? <ActionButton label="取消编辑" onClick={resetForm} /> : null}
          </div>
        </form>
      </section>

      <section className={styles.sectionCard}>
        {credentialsQuery.isPending ? (
          <LoadingState title="正在加载凭据列表" message="正在同步仓库认证记录。" />
        ) : credentialsQuery.isError ? (
          <ErrorState
            title="凭据列表加载失败"
            message={getErrorMessage(credentialsQuery.error)}
            onRetry={() => credentialsQuery.refetch()}
          />
        ) : !credentialsQuery.data?.length ? (
          <EmptyState title="暂无凭据" message="请先创建至少一个仓库凭据。" />
        ) : (
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
                {credentialsQuery.data.map((item) => (
                  <tr key={item.id}>
                    <td>{item.registryHost}</td>
                    <td>{item.username}</td>
                    <td>{item.secretConfigured ? "是" : "否"}</td>
                    <td>{formatDateTime(item.updatedAt)}</td>
                    <td>
                      <div className={styles.buttonRow}>
                        <ActionButton
                          label="编辑"
                          onClick={() => {
                            setEditing(item);
                            setForm({
                              registryHost: item.registryHost,
                              username: item.username,
                              secret: "",
                            });
                          }}
                        />
                        <ActionButton
                          label="删除"
                          pending={deleteMutation.isPending}
                          pendingLabel="删除中"
                          onClick={() => deleteMutation.mutate(item.id)}
                        />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
