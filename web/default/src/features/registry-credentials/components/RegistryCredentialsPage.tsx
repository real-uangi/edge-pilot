import { useId, useRef, useState } from "react";
import {
  useMutation,
  useMutationState,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { formatDateTime } from "../../../shared/lib/format";
import { ActionButton } from "../../../shared/components/ActionButton";
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { StatusPill } from "../../../shared/components/StatusPill";
import { Modal } from "../../../shared/components/Modal";
import { useDialog } from "../../../shared/components/DialogProvider";
import { registryCredentialsApi } from "../api";
import type { RegistryCredentialRecord } from "../types";
import styles from "../../../styles/admin.module.css";

const credentialSchema = z.object({
  registryHost: z.string().trim().min(1, "请输入仓库地址"),
  username: z.string().trim().min(1, "请输入用户名"),
  secret: z.string().trim().min(1, "请输入密码或令牌"),
});
type CredentialForm = z.infer<typeof credentialSchema>;
const emptyForm: CredentialForm = {
  registryHost: "",
  username: "",
  secret: "",
};

export function RegistryCredentialsPage() {
  const fieldId = useId();
  const queryClient = useQueryClient();
  const dialog = useDialog();
  const [editing, setEditing] = useState<RegistryCredentialRecord | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const submissionRef = useRef<{ id?: string; input: CredentialForm } | null>(
    null,
  );
  const [search, setSearch] = useState("");
  const [feedback, setFeedback] = useState<{
    message: string;
    error: boolean;
  } | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CredentialForm>({
    resolver: zodResolver(credentialSchema),
    defaultValues: emptyForm,
  });
  const credentialsQuery = useQuery({
    queryKey: ["registry-credentials"],
    queryFn: registryCredentialsApi.list,
  });
  const saveMutation = useMutation({
    mutationFn: async () => {
      const submission = submissionRef.current!;
      try {
        return submission.id
          ? await registryCredentialsApi.update(submission.id, submission.input)
          : await registryCredentialsApi.create(submission.input);
      } finally {
        submissionRef.current = null;
      }
    },
    onSuccess: async (output) => {
      setFormOpen(false);
      reset(emptyForm);
      setEditing(null);
      setFeedback({
        message: `已保存「${output.registryHost}」的凭据`,
        error: false,
      });
      await queryClient.invalidateQueries({
        queryKey: ["registry-credentials"],
      });
    },
  });
  const deleteMutation = useMutation({
    mutationKey: ["registry-credential-delete"],
    mutationFn: (item: RegistryCredentialRecord) =>
      registryCredentialsApi.delete(item.id),
    onMutate: () => setFeedback(null),
    onSuccess: async (_, item) => {
      setFeedback({
        message: `已删除「${item.registryHost}」的凭据`,
        error: false,
      });
      await queryClient.invalidateQueries({
        queryKey: ["registry-credentials"],
      });
    },
    onError: (error, item) =>
      setFeedback({
        message: `${item.registryHost}：${getErrorMessage(error)}`,
        error: true,
      }),
  });
  const pendingIds = useMutationState({
    filters: { mutationKey: ["registry-credential-delete"], status: "pending" },
    select: (mutation) =>
      (mutation.state.variables as RegistryCredentialRecord).id,
  });
  const openForm = (item: RegistryCredentialRecord | null) => {
    if (saveMutation.isPending) return;
    setEditing(item);
    reset(
      item
        ? {
            registryHost: item.registryHost,
            username: item.username,
            secret: "",
          }
        : emptyForm,
    );
    saveMutation.reset();
    setFormOpen(true);
  };
  const closeForm = () => {
    if (saveMutation.isPending) return;
    submissionRef.current = null;
    setFormOpen(false);
    setEditing(null);
    reset(emptyForm);
    saveMutation.reset();
  };
  const handleDelete = async (item: RegistryCredentialRecord) => {
    if (
      await dialog.confirm({
        title: "删除仓库凭据",
        message: `确认删除「${item.registryHost}」的凭据？后续私有镜像拉取可能受影响。`,
        confirmText: "删除凭据",
        danger: true,
      })
    )
      deleteMutation.mutate(item);
  };
  const query = search.trim().toLowerCase();
  const credentials = (credentialsQuery.data ?? []).filter(
    (item) =>
      !query ||
      [item.registryHost, item.username].some((value) =>
        value.toLowerCase().includes(query),
      ),
  );

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <h1 className={styles.sectionTitle}>镜像仓库凭据</h1>
        <div className={styles.buttonRow}>
          <ActionButton
            label="刷新"
            pending={credentialsQuery.isFetching}
            onClick={() => void credentialsQuery.refetch()}
          />
          <ActionButton
            label="新建凭据"
            disabled={saveMutation.isPending}
            variant="primary"
            onClick={() => openForm(null)}
          />
        </div>
      </header>
      {feedback ? (
        <InlineNotice
          message={feedback.message}
          tone={feedback.error ? "error" : "info"}
        />
      ) : null}
      <section className={styles.sectionCard}>
        <div className={styles.listToolbar}>
          <label className={styles.searchField}>
            <span className={styles.visuallyHidden}>搜索仓库凭据</span>
            <input
              className={styles.input}
              type="search"
              placeholder="搜索仓库地址或用户名"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <span className={styles.resultCount} aria-live="polite">
            {credentials.length} 个仓库
          </span>
        </div>
        <QueryRefreshNotice
          query={credentialsQuery}
          onRetry={() => void credentialsQuery.refetch()}
        />
        {credentialsQuery.isPending ? (
          <LoadingState title="正在加载仓库凭据" />
        ) : credentialsQuery.isError && !credentialsQuery.data ? (
          <ErrorState
            title="仓库凭据加载失败"
            message={getErrorMessage(credentialsQuery.error)}
            onRetry={() => void credentialsQuery.refetch()}
          />
        ) : !credentialsQuery.data?.length ? (
          <EmptyState
            title="暂无仓库凭据"
            actionLabel="新建凭据"
            onRetry={() => openForm(null)}
          />
        ) : !credentials.length ? (
          <EmptyState
            title="没有匹配的仓库凭据"
            actionLabel="清除搜索"
            onRetry={() => setSearch("")}
          />
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.responsiveTable}>
              <thead>
                <tr>
                  <th>仓库地址</th>
                  <th>用户名</th>
                  <th>密钥</th>
                  <th>更新时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {credentials.map((item) => (
                  <tr key={item.id}>
                    <td data-label="仓库地址">
                      <code>{item.registryHost}</code>
                    </td>
                    <td data-label="用户名">{item.username}</td>
                    <td data-label="密钥">
                      <StatusPill
                        label={item.secretConfigured ? "已配置" : "未配置"}
                        tone={item.secretConfigured ? "success" : "warning"}
                      />
                    </td>
                    <td data-label="更新时间">
                      {formatDateTime(item.updatedAt)}
                    </td>
                    <td data-label="操作">
                      <div className={styles.buttonRow}>
                        <ActionButton
                          label="编辑"
                          disabled={pendingIds.includes(item.id)}
                          onClick={() => openForm(item)}
                        />
                        <ActionButton
                          label="删除"
                          variant="danger"
                          pending={pendingIds.includes(item.id)}
                          pendingLabel="删除中"
                          onClick={() => void handleDelete(item)}
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
      {formOpen ? (
        <Modal
          title={
            editing ? `编辑凭据 · ${editing.registryHost}` : "新建仓库凭据"
          }
          closeDisabled={saveMutation.isPending}
          onClose={closeForm}
        >
          <form
            className={styles.formStack}
            noValidate
            onSubmit={handleSubmit((input) => {
              if (saveMutation.isPending) return;
              submissionRef.current = { id: editing?.id, input };
              saveMutation.mutate();
            })}
          >
            <label className={styles.field}>
              <span className={styles.label}>仓库地址 *</span>
              <input
                className={styles.input}
                {...register("registryHost")}
                placeholder="ghcr.io"
                autoComplete="off"
                aria-required="true"
                aria-invalid={Boolean(errors.registryHost)}
                aria-describedby={
                  errors.registryHost ? `${fieldId}-host-error` : undefined
                }
              />
              {errors.registryHost ? (
                <span
                  className={styles.fieldError}
                  id={`${fieldId}-host-error`}
                >
                  {errors.registryHost.message}
                </span>
              ) : null}
            </label>
            <label className={styles.field}>
              <span className={styles.label}>用户名 *</span>
              <input
                className={styles.input}
                {...register("username")}
                autoComplete="off"
                aria-required="true"
                aria-invalid={Boolean(errors.username)}
                aria-describedby={
                  errors.username ? `${fieldId}-username-error` : undefined
                }
              />
              {errors.username ? (
                <span
                  className={styles.fieldError}
                  id={`${fieldId}-username-error`}
                >
                  {errors.username.message}
                </span>
              ) : null}
            </label>
            <label className={styles.field}>
              <span className={styles.label}>密码或令牌 *</span>
              <input
                className={styles.input}
                {...register("secret")}
                type="password"
                autoComplete="new-password"
                aria-required="true"
                aria-invalid={Boolean(errors.secret)}
                aria-describedby={
                  `${editing ? `${fieldId}-secret-hint ` : ""}${errors.secret ? `${fieldId}-secret-error` : ""}`.trim() ||
                  undefined
                }
              />
              {editing ? (
                <span className={styles.hint} id={`${fieldId}-secret-hint`}>
                  更新凭据时需重新输入密码或令牌。
                </span>
              ) : null}
              {errors.secret ? (
                <span
                  className={styles.fieldError}
                  id={`${fieldId}-secret-error`}
                >
                  {errors.secret.message}
                </span>
              ) : null}
            </label>
            {saveMutation.isError ? (
              <InlineNotice message={getErrorMessage(saveMutation.error)} />
            ) : null}
            <div className={styles.formActions}>
              <ActionButton
                label="取消"
                disabled={saveMutation.isPending}
                onClick={closeForm}
              />
              <ActionButton
                label={editing ? "保存凭据" : "创建凭据"}
                variant="primary"
                type="submit"
                pending={saveMutation.isPending}
                pendingLabel="保存中"
              />
            </div>
          </form>
        </Modal>
      ) : null}
    </div>
  );
}
