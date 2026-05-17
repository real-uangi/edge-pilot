import { useEffect, useRef, useState } from "react";
import AnsiToHtml from "ansi-to-html";
import { instancesApi } from "../api";
import styles from "../../../styles/admin.module.css";

interface ContainerLogDialogProps {
  agentId: string;
  containerId: string;
  containerName: string;
  onClose: () => void;
}

export function ContainerLogDialog({ agentId, containerId, containerName, onClose }: ContainerLogDialogProps) {
  const [logs, setLogs] = useState<Array<{ data: string; stderr: boolean; id: number }>>([]);
  const [filterStderr, setFilterStderr] = useState(true);
  const [filterStdout, setFilterStdout] = useState(true);
  const [keyword, setKeyword] = useState("");
  const [connected, setConnected] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const ansiConverter = useRef(new AnsiToHtml());
  const nextId = useRef(0);

  useEffect(() => {
    setConnected(true);
    const cleanup = instancesApi.streamLogs(
      agentId,
      containerId,
      (chunk) => {
        setLogs((prev) => [
          ...prev,
          { data: chunk.data, stderr: chunk.stderr, id: nextId.current++ },
        ]);
      },
      () => {
        setConnected(false);
      }
    );
    return cleanup;
  }, [agentId, containerId]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  const filteredLogs = logs.filter((log) => {
    if (log.stderr && !filterStderr) return false;
    if (!log.stderr && !filterStdout) return false;
    if (keyword && !log.data.toLowerCase().includes(keyword.toLowerCase())) return false;
    return true;
  });

  return (
    <div className={styles.dialogOverlay} onClick={onClose}>
      <div className={styles.dialog} onClick={(e) => e.stopPropagation()}>
        <div className={styles.dialogHeader}>
          <div>
            <h3>实时日志 - {containerName}</h3>
            <span className={styles.connectionStatus}>
              {connected ? "● 已连接" : "● 已断开"}
            </span>
          </div>
          <button className={styles.closeButton} onClick={onClose} type="button">
            ×
          </button>
        </div>

        <div className={styles.dialogToolbar}>
          <label className={styles.toolbarLabel}>
            <input
              type="checkbox"
              checked={filterStdout}
              onChange={(e) => setFilterStdout(e.target.checked)}
            />
            stdout
          </label>
          <label className={styles.toolbarLabel}>
            <input
              type="checkbox"
              checked={filterStderr}
              onChange={(e) => setFilterStderr(e.target.checked)}
            />
            stderr
          </label>
          <input
            type="text"
            className={styles.toolbarInput}
            placeholder="过滤关键字..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
          <button className={styles.toolbarButton} onClick={() => setLogs([])} type="button">
            清空
          </button>
        </div>

        <div className={styles.logContent} ref={scrollRef}>
          {filteredLogs.length === 0 ? (
            <div className={styles.logEmpty}>暂无日志</div>
          ) : (
            filteredLogs.map((log) => (
              <div
                key={log.id}
                className={log.stderr ? styles.logStderr : styles.logStdout}
                dangerouslySetInnerHTML={{
                  __html: ansiConverter.current.toHtml(log.data),
                }}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
