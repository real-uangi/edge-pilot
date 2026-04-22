import { useEffect, useMemo, useState, type PropsWithChildren } from "react";
import { useQuery } from "@tanstack/react-query";
import { NavLink } from "react-router-dom";
import { api } from "../lib/api";
import styles from "./AppShell.module.css";

const navItems = [
  { to: "/", label: "总览", end: true },
  { to: "/system-performance", label: "系统性能" },
  { to: "/services", label: "服务" },
  { to: "/registry-credentials", label: "镜像仓库" },
  { to: "/agents", label: "节点" },
  { to: "/releases", label: "发布" },
];

interface AppShellProps extends PropsWithChildren {
  username: string;
  loggingOut: boolean;
  onLogout: () => void;
}

const rightRailCollapsedStorageKey = "ep:right-rail-collapsed";

export function AppShell({ username, loggingOut, onLogout, children }: AppShellProps) {
  const [leftRailOpen, setLeftRailOpen] = useState(false);
  const [rightRailOpen, setRightRailOpen] = useState(false);
  const [rightRailCollapsed, setRightRailCollapsed] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    try {
      return window.localStorage.getItem(rightRailCollapsedStorageKey) === "1";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    try {
      window.localStorage.setItem(rightRailCollapsedStorageKey, rightRailCollapsed ? "1" : "0");
    } catch {
      // no-op
    }
  }, [rightRailCollapsed]);

  const overviewQuery = useQuery({
    queryKey: ["overview"],
    queryFn: api.overview,
    refetchInterval: 10000,
  });

  const summary = useMemo(() => {
    const data = overviewQuery.data;
    if (!data) {
      return null;
    }
    const onlineAgents = data.agents.filter((item) => item.online).length;
    const failedReleases = data.recentReleases.filter((item) => item.status === 7).length;
    const activeReleases = data.recentReleases.filter((item) => item.isActive).length;
    const disabledServices = data.services.filter((item) => item.enabled === false).length;

    return {
      onlineAgents,
      totalAgents: data.agents.length,
      activeInstances: data.activeInstances,
      activeReleases,
      failedReleases,
      disabledServices,
      recentReleases: data.recentReleases.slice(0, 5),
    };
  }, [overviewQuery.data]);

  const closeDrawers = () => {
    setLeftRailOpen(false);
    setRightRailOpen(false);
  };

  return (
    <div className={`${styles.shell} ${rightRailCollapsed ? styles.shellRightRailCollapsed : ""}`}>
      <header className={styles.mobileHeader}>
        <div className={styles.mobileBrand}>Edge Pilot Control Plane</div>
        <div className={styles.mobileActions}>
          <button
            className={styles.mobileToggle}
            onClick={() => {
              setRightRailOpen(false);
              setLeftRailOpen((current) => !current);
            }}
            type="button"
          >
            菜单
          </button>
          <button
            className={styles.mobileToggle}
            onClick={() => {
              setLeftRailOpen(false);
              setRightRailOpen((current) => !current);
            }}
            type="button"
          >
            健康
          </button>
        </div>
      </header>

      <div className={styles.layout}>
        <aside className={`${styles.leftRail} ${leftRailOpen ? styles.leftRailOpen : ""}`}>
          <div className={styles.railBrand}>
            <span className={styles.brand}>Edge Pilot</span>
            <span className={styles.meta}>Control Plane</span>
          </div>

          <nav className={styles.nav}>
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                end={item.end}
                to={item.to}
                className={({ isActive }) => (isActive ? styles.navActive : styles.navLink)}
                onClick={closeDrawers}
              >
                {item.label}
              </NavLink>
            ))}
          </nav>

          <div className={styles.sessionCard}>
            <span className={styles.meta}>当前账号</span>
            <span className={styles.user}>{username}</span>
            <button className={styles.logout} disabled={loggingOut} onClick={onLogout} type="button">
              {loggingOut ? "退出中" : "退出登录"}
            </button>
          </div>
        </aside>

        <main className={styles.stage}>
          <div className={styles.stageInner}>{children}</div>
        </main>

        <aside
          className={`${styles.rightRail} ${rightRailOpen ? styles.rightRailOpen : ""} ${
            rightRailCollapsed ? styles.rightRailCollapsed : ""
          }`}
          onClick={rightRailCollapsed ? () => setRightRailCollapsed(false) : undefined}
          onKeyDown={
            rightRailCollapsed
              ? (event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    setRightRailCollapsed(false);
                  }
                }
              : undefined
          }
          role={rightRailCollapsed ? "button" : undefined}
          tabIndex={rightRailCollapsed ? 0 : undefined}
          aria-label={rightRailCollapsed ? "展开系统健康" : undefined}
        >
          {!rightRailCollapsed ? (
            <button
              className={styles.rightRailEdgeToggle}
              onClick={() => setRightRailCollapsed(true)}
              type="button"
              aria-label="收起系统健康"
              title="收起系统健康"
            >
              收起
            </button>
          ) : null}

          <div className={styles.rightRailExpanded}>
            <div className={styles.rightHeader}>
              <span className={styles.meta}>Global Health</span>
              <h2 className={styles.rightTitle}>系统健康</h2>
            </div>

            {overviewQuery.isPending ? (
              <div className={styles.healthMeta}>正在读取运行指标...</div>
            ) : overviewQuery.isError ? (
              <div className={styles.healthMeta}>健康数据暂时不可用</div>
            ) : summary ? (
              <>
                <div className={styles.healthGrid}>
                  <article className={styles.healthCard}>
                    <span className={styles.healthLabel}>在线节点</span>
                    <span className={styles.healthValue}>
                      {summary.onlineAgents}/{summary.totalAgents}
                    </span>
                    <span className={styles.healthMeta}>10 秒刷新</span>
                  </article>
                  <article className={styles.healthCard}>
                    <span className={styles.healthLabel}>活动发布</span>
                    <span className={styles.healthValue}>{summary.activeReleases}</span>
                    <span className={styles.healthMeta}>失败 {summary.failedReleases}</span>
                  </article>
                  <article className={styles.healthCard}>
                    <span className={styles.healthLabel}>运行实例</span>
                    <span className={styles.healthValue}>{summary.activeInstances}</span>
                    <span className={styles.healthMeta}>受控实例</span>
                  </article>
                  <article className={styles.healthCard}>
                    <span className={styles.healthLabel}>停用服务</span>
                    <span className={styles.healthValue}>{summary.disabledServices}</span>
                    <span className={styles.healthMeta}>关注配置漂移</span>
                  </article>
                </div>

                <div className={styles.healthList}>
                  <span className={styles.meta}>最近发布</span>
                  {!summary.recentReleases.length ? (
                    <div className={styles.healthMeta}>暂无发布记录</div>
                  ) : (
                    summary.recentReleases.map((release) => (
                      <div className={styles.healthItem} key={release.id}>
                        <div>
                          <div className={styles.healthItemTitle}>#{release.id.slice(0, 8)}</div>
                          <div className={styles.healthItemMeta}>{release.imageRepo + ":" + release.imageTag}</div>
                        </div>
                        <span
                          className={`${styles.statusLine} ${
                            release.status === 7
                              ? styles.statusDown
                              : release.isActive
                                ? styles.statusWarn
                                : styles.statusUp
                          }`}
                        >
                          {release.status === 7 ? "失败" : release.isActive ? "进行中" : "稳定"}
                        </span>
                      </div>
                    ))
                  )}
                </div>
              </>
            ) : null}
          </div>

          <div className={styles.rightRailCompact}>
            <div className={styles.rightRailCompactTitle}>系统健康</div>
          </div>
        </aside>
      </div>

      {leftRailOpen || rightRailOpen ? (
        <button aria-label="关闭抽屉" className={styles.railBackdrop} onClick={closeDrawers} type="button" />
      ) : null}
    </div>
  );
}
