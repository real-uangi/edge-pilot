import { useSearchParams } from "react-router-dom";
import styles from "./SchedulerTabs.module.css";

export type SchedulerTab = "jobs" | "history" | "executors";

const tabs: { key: SchedulerTab; label: string }[] = [
  { key: "jobs", label: "任务定义" },
  { key: "history", label: "执行历史" },
  { key: "executors", label: "执行器凭证" },
];

interface Props {
  active: SchedulerTab;
}

export function SchedulerTabs({ active }: Props) {
  const [, setSearchParams] = useSearchParams();
  return (
    <nav className={styles.tabBar}>
      {tabs.map((tab) => (
        <button
          key={tab.key}
          className={tab.key === active ? styles.tabActive : styles.tab}
          onClick={() => setSearchParams({ tab: tab.key })}
          type="button"
        >
          {tab.label}
        </button>
      ))}
    </nav>
  );
}
