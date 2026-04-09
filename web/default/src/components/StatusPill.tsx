import styles from "../styles/admin.module.css";

type Tone = "default" | "success" | "danger" | "warning" | "dark";

export function StatusPill({
  label,
  tone = "default",
}: {
  label: string;
  tone?: Tone;
}) {
  return <span className={`${styles.statusPill} ${styles[`tone${tone}`]}`}>{label}</span>;
}
