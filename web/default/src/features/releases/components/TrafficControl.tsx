import { useState, useEffect } from "react";
import { ActionButton } from "../../../shared/components/ActionButton";
import { InlineNotice } from "../../../shared/components/StateBlocks";
import type { ReleaseRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface TrafficControlProps {
  release: ReleaseRecord;
  setTrafficPending: boolean;
  onSetTraffic: (percent: number) => void;
}

export function TrafficControl({ release, setTrafficPending, onSetTraffic }: TrafficControlProps) {
  const [trafficDraft, setTrafficDraft] = useState(release.trafficPercent);
  const canAdjustTraffic = release.status === 4;

  useEffect(() => {
    setTrafficDraft(release.trafficPercent);
  }, [release.trafficPercent]);

  return (
    <section className={styles.sectionCard}>
      <div className={styles.buttonRow}>
        {[0, 10, 30, 50, 80, 100].map((value) => (
          <ActionButton
            key={value}
            label={`${value}%`}
            variant={value === release.trafficPercent ? "primary" : "secondary"}
            pending={setTrafficPending && trafficDraft === value}
            disabled={setTrafficPending || !canAdjustTraffic}
            onClick={() => {
              setTrafficDraft(value);
              onSetTraffic(value);
            }}
          />
        ))}
        <input
          type="number"
          min={0}
          max={100}
          value={trafficDraft}
          className={styles.input}
          onChange={(event) => {
            const next = Number(event.target.value);
            if (!Number.isFinite(next)) {
              setTrafficDraft(0);
              return;
            }
            setTrafficDraft(Math.max(0, Math.min(100, Math.round(next))));
          }}
        />
        <ActionButton
          label="应用比例"
          pending={setTrafficPending}
          variant="primary"
          disabled={setTrafficPending || !canAdjustTraffic}
          onClick={() => onSetTraffic(trafficDraft)}
        />
      </div>
      {release.verificationUrl ? (
        <InlineNotice
          message="目标版本验证链接已就绪"
          tone="info"
        />
      ) : null}
    </section>
  );
}
