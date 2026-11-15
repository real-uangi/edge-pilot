import styles from "../../styles/admin.module.css";
import { getErrorMessage } from "../lib/api-client";
import { formatDateTime } from "../lib/format";

interface StateProps {
  title: string;
  message?: string;
  actionLabel?: string;
  onRetry?: () => void;
}

export function LoadingState({
  title,
  message,
}: Pick<StateProps, "title" | "message">) {
  return (
    <section className={styles.stateCard} aria-live="polite" aria-busy="true">
      <h2 className={styles.stateTitle}>{title}</h2>
      {message ? <p className={styles.stateMessage}>{message}</p> : null}
      <div className={styles.skeletonLines} aria-hidden="true">
        <span className={styles.skeletonLine} />
        <span className={styles.skeletonLine} />
        <span className={styles.skeletonLine} />
      </div>
    </section>
  );
}

export function QueryRefreshNotice({
  query,
  onRetry,
}: {
  query: {
    isRefetchError: boolean;
    error: unknown;
    dataUpdatedAt: number;
    isFetching: boolean;
  };
  onRetry: () => void;
}) {
  return (
    <div className={styles.refreshMeta}>
      <span>
        {query.isFetching
          ? "正在刷新"
          : query.dataUpdatedAt
            ? `更新于 ${formatDateTime(new Date(query.dataUpdatedAt).toISOString())}`
            : "尚未更新"}
      </span>
      {query.isRefetchError ? (
        <div role="alert" className={styles.buttonRow}>
          <span className={styles.inlineError}>
            刷新失败：{getErrorMessage(query.error)}
          </span>
          <button
            type="button"
            className={styles.ghostButton}
            disabled={query.isFetching}
            onClick={onRetry}
          >
            重试
          </button>
        </div>
      ) : null}
    </div>
  );
}

export function ErrorState({
  title,
  message,
  actionLabel = "重试",
  onRetry,
}: StateProps) {
  return (
    <section className={styles.stateCard} role="alert">
      <h2 className={styles.stateTitle}>{title}</h2>
      {message ? <p className={styles.stateMessage}>{message}</p> : null}
      {onRetry ? (
        <div className={styles.stateActions}>
          <button
            className={styles.secondaryButton}
            onClick={onRetry}
            type="button"
          >
            {actionLabel}
          </button>
        </div>
      ) : null}
    </section>
  );
}

export function EmptyState({
  title,
  message,
  actionLabel,
  onRetry,
}: StateProps) {
  return (
    <section className={styles.stateCard} aria-live="polite">
      <h2 className={styles.stateTitle}>{title}</h2>
      {message ? <p className={styles.stateMessage}>{message}</p> : null}
      {onRetry && actionLabel ? (
        <div className={styles.stateActions}>
          <button
            className={styles.secondaryButton}
            onClick={onRetry}
            type="button"
          >
            {actionLabel}
          </button>
        </div>
      ) : null}
    </section>
  );
}

export function InlineNotice({
  message,
  tone = "error",
}: {
  message: string;
  tone?: "error" | "info";
}) {
  return (
    <div
      className={`${styles.inlineNotice} ${tone === "error" ? styles.inlineNoticeError : styles.inlineNoticeInfo}`}
      role={tone === "error" ? "alert" : "status"}
    >
      {message}
    </div>
  );
}
