import styles from "../../styles/admin.module.css";

interface StateProps {
  title: string;
  message?: string;
  actionLabel?: string;
  onRetry?: () => void;
}

export function LoadingState({ title, message }: Pick<StateProps, "title" | "message">) {
  return (
    <section className={styles.stateCard} aria-live="polite">
      <span className={styles.loadingDot} aria-hidden="true" />
      <h2 className={styles.stateTitle}>{title}</h2>
      {message ? <p className={styles.stateMessage}>{message}</p> : null}
    </section>
  );
}

export function ErrorState({ title, message, actionLabel = "重试", onRetry }: StateProps) {
  return (
    <section className={styles.stateCard} role="alert">
      <h2 className={styles.stateTitle}>{title}</h2>
      {message ? <p className={styles.stateMessage}>{message}</p> : null}
      {onRetry ? (
        <div className={styles.stateActions}>
          <button className={styles.secondaryButton} onClick={onRetry} type="button">
            {actionLabel}
          </button>
        </div>
      ) : null}
    </section>
  );
}

export function EmptyState({ title, message, actionLabel, onRetry }: StateProps) {
  return (
    <section className={styles.stateCard} aria-live="polite">
      <h2 className={styles.stateTitle}>{title}</h2>
      {message ? <p className={styles.stateMessage}>{message}</p> : null}
      {onRetry && actionLabel ? (
        <div className={styles.stateActions}>
          <button className={styles.secondaryButton} onClick={onRetry} type="button">
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
      role={tone === "error" ? "alert" : undefined}
    >
      {message}
    </div>
  );
}
