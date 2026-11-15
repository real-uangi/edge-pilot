import styles from "../../styles/admin.module.css";

export function FullscreenState({ title }: { title: string }) {
  return (
    <div className={styles.fullscreenState} role="status" aria-live="polite">
      <span className={styles.loadingDot} aria-hidden="true" />
      <span>{title}</span>
    </div>
  );
}
