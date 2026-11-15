import { useState, useEffect } from "react";
import { ActionButton } from "../../../shared/components/ActionButton";
import type { ReleaseRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface TrafficControlProps {
  release: ReleaseRecord;
  disabled: boolean;
  setTrafficPending: boolean;
  onSetTraffic: (percent: number) => void;
}

export function TrafficControl({
  release,
  disabled,
  setTrafficPending,
  onSetTraffic,
}: TrafficControlProps) {
  const [trafficDraft, setTrafficDraft] = useState(
    String(release.trafficPercent),
  );
  const percent = Number(trafficDraft);
  const valid =
    trafficDraft !== "" &&
    Number.isInteger(percent) &&
    percent >= 0 &&
    percent <= 100;

  useEffect(() => {
    setTrafficDraft(String(release.trafficPercent));
  }, [release.trafficPercent]);

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionSubtitle}>流量控制</h2>
        <span className={styles.mono}>当前 {release.trafficPercent}%</span>
      </div>
      <div
        className={styles.trafficTrack}
        role="meter"
        aria-label="目标版本当前流量比例"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={release.trafficPercent}
      >
        <span
          className={styles.trafficFill}
          style={{ width: `${release.trafficPercent}%` }}
        />
      </div>
      <div
        className={styles.buttonRow}
        role="group"
        aria-label="选择待应用流量比例"
      >
        {[0, 10, 30, 50, 80, 100].map((value) => (
          <ActionButton
            key={value}
            label={`${value}%`}
            aria-pressed={trafficDraft === String(value)}
            disabled={disabled}
            onClick={() => setTrafficDraft(String(value))}
          />
        ))}
      </div>
      <form
        className={styles.toolbar}
        onSubmit={(event) => {
          event.preventDefault();
          if (valid && !disabled) onSetTraffic(percent);
        }}
      >
        <label className={styles.field}>
          <span className={styles.label}>待应用比例（%）</span>
          <input
            type="number"
            min={0}
            max={100}
            step={1}
            required
            value={trafficDraft}
            className={styles.input}
            disabled={disabled}
            aria-invalid={!valid}
            aria-describedby={!valid ? "traffic-error" : undefined}
            onChange={(event) => setTrafficDraft(event.target.value)}
          />
          {!valid ? (
            <span className={styles.fieldError} id="traffic-error">
              请输入 0–100 的整数
            </span>
          ) : null}
        </label>
        <ActionButton
          label="应用比例"
          pending={setTrafficPending}
          variant="primary"
          disabled={disabled || !valid || percent === release.trafficPercent}
          type="submit"
        />
      </form>
    </section>
  );
}
