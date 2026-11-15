import { useEffect, useRef, useState } from "react";
import AnsiToHtml from "ansi-to-html";
import { instancesApi } from "../api";
import { Modal } from "../../../shared/components/Modal";
import { ActionButton } from "../../../shared/components/ActionButton";
import { StatusPill } from "../../../shared/components/StatusPill";
import styles from "../../../styles/admin.module.css";
import logsStyle from "./ContainerLogDialog.module.css";

const MAX_LOG_CHUNKS = 5000;
function escapeHtml(text: string) {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}
interface ContainerLogDialogProps {
  agentId: string;
  containerId: string;
  containerName: string;
  onClose: () => void;
}

export function ContainerLogDialog({
  agentId,
  containerId,
  containerName,
  onClose,
}: ContainerLogDialogProps) {
  const [logs, setLogs] = useState<
    Array<{ data: string; stderr: boolean; id: number }>
  >([]);
  const [stdout, setStdout] = useState(true);
  const [stderr, setStderr] = useState(true);
  const [keyword, setKeyword] = useState("");
  const [connection, setConnection] = useState<
    "connecting" | "connected" | "reconnecting" | "closed"
  >("connecting");
  const [reconnect, setReconnect] = useState(0);
  const [session, setSession] = useState(0);
  const [follow, setFollow] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const converter = useRef(new AnsiToHtml({ fg: "#e1e6ed", bg: "#171c23" }));
  const nextId = useRef(0);

  useEffect(() => {
    setConnection("connecting");
    return instancesApi.streamLogs(
      agentId,
      containerId,
      (chunk) => {
        setLogs((previous) =>
          [...previous, { ...chunk, id: nextId.current++ }].slice(
            -MAX_LOG_CHUNKS,
          ),
        );
      },
      (closed) => setConnection(closed ? "closed" : "reconnecting"),
      () => {
        setConnection("connected");
        setLogs([]);
        setSession((value) => value + 1);
      },
    );
  }, [agentId, containerId, reconnect]);
  useEffect(() => {
    if (follow && scrollRef.current)
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [logs, follow, keyword, stdout, stderr]);
  const filtered = logs.filter(
    (log) =>
      (log.stderr ? stderr : stdout) &&
      log.data.toLowerCase().includes(keyword.toLowerCase()),
  );
  const connectionLabel = {
    connecting: "正在连接",
    connected: "已连接",
    reconnecting: "连接中断，正在重连",
    closed: "连接已关闭",
  }[connection];

  return (
    <Modal
      title={`实时日志 · ${containerName}`}
      onClose={onClose}
      variant="wide"
    >
      <div className={styles.page}>
        <div className={styles.toolbar}>
          <StatusPill
            label={connectionLabel}
            tone={
              connection === "connected"
                ? "success"
                : connection === "closed"
                  ? "danger"
                  : "warning"
            }
          />
          <label className={styles.checkboxRow}>
            <input
              type="checkbox"
              checked={stdout}
              onChange={(event) => setStdout(event.target.checked)}
            />
            stdout
          </label>
          <label className={styles.checkboxRow}>
            <input
              type="checkbox"
              checked={stderr}
              onChange={(event) => setStderr(event.target.checked)}
            />
            stderr
          </label>
          <label className={styles.checkboxRow}>
            <input
              type="checkbox"
              checked={follow}
              onChange={(event) => setFollow(event.target.checked)}
            />
            跟随日志
          </label>
        </div>
        <div className={styles.toolbar}>
          <label className={logsStyle.search}>
            <span className={styles.visuallyHidden}>过滤日志关键字</span>
            <input
              type="search"
              className={styles.input}
              placeholder="过滤日志关键字"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
            />
          </label>
          <ActionButton
            label="回到底部"
            onClick={() => {
              setFollow(true);
              if (scrollRef.current)
                scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
            }}
          />
          <ActionButton label="清空" onClick={() => setLogs([])} />
          {connection === "closed" || connection === "reconnecting" ? (
            <ActionButton
              label="重新连接"
              onClick={() => setReconnect((value) => value + 1)}
            />
          ) : null}
        </div>
        <div
          className={logsStyle.content}
          ref={scrollRef}
          role="region"
          aria-label="容器日志"
          tabIndex={0}
          onScroll={() => {
            const element = scrollRef.current!;
            if (
              element.scrollHeight - element.scrollTop - element.clientHeight >
              24
            )
              setFollow(false);
          }}
        >
          {!filtered.length ? (
            <div className={logsStyle.empty}>
              {logs.length ? "暂无匹配日志" : "暂无日志"}
            </div>
          ) : (
            filtered.map((log) => (
              <div
                key={log.id}
                className={log.stderr ? logsStyle.stderr : logsStyle.stdout}
                dangerouslySetInnerHTML={{
                  __html: converter.current.toHtml(escapeHtml(log.data)),
                }}
              />
            ))
          )}
        </div>
        <span className={styles.hint}>
          {filtered.length} 条匹配日志 · 最多保留 {MAX_LOG_CHUNKS} 条
        </span>
        {session > 1 ? (
          <span className={styles.hint}>
            已重新连接，当前显示本次连接的日志。
          </span>
        ) : null}
      </div>
    </Modal>
  );
}
