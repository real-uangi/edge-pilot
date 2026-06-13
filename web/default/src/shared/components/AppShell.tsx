import { useEffect, useMemo, useState, type PropsWithChildren } from "react";
import { useQuery } from "@tanstack/react-query";
import { NavLink, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  Server,
  HardDrive,
  Rocket,
  Clock,
  Activity,
  ChevronDown,
  ChevronRight,
} from "lucide-react";
import { dashboardApi } from "../../features/dashboard/api";
import styles from "./AppShell.module.css";

type NavItem = { to: string; label: string; end?: boolean };

type NavGroup =
  | {
      key: string;
      label: string;
      icon: React.ComponentType<{ size?: number; className?: string }>;
      type: "link";
      to: string;
      end?: boolean;
    }
  | {
      key: string;
      label: string;
      icon: React.ComponentType<{ size?: number; className?: string }>;
      type: "group";
      items: NavItem[];
    };

const navGroups: NavGroup[] = [
  {
    key: "overview",
    label: "总览",
    icon: LayoutDashboard,
    type: "link",
    to: "/",
    end: true,
  },
  {
    key: "service",
    label: "服务管理",
    icon: Server,
    type: "group",
    items: [
      { to: "/services", label: "服务" },
      { to: "/registry-credentials", label: "镜像仓库" },
    ],
  },
  {
    key: "infra",
    label: "基础设施",
    icon: HardDrive,
    type: "group",
    items: [
      { to: "/agents", label: "节点" },
      { to: "/instances", label: "受管实例" },
    ],
  },
  {
    key: "release",
    label: "发布管理",
    icon: Rocket,
    type: "group",
    items: [{ to: "/releases", label: "发布" }],
  },
  {
    key: "scheduler",
    label: "任务调度",
    icon: Clock,
    type: "group",
    items: [
      { to: "/scheduler", label: "定时任务" },
      { to: "/scheduler/history", label: "执行历史" },
      { to: "/scheduler/executors", label: "执行器" },
    ],
  },
  {
    key: "monitor",
    label: "系统监控",
    icon: Activity,
    type: "group",
    items: [{ to: "/system-performance", label: "系统性能" }],
  },
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

  const location = useLocation();

  useEffect(() => {
    try {
      window.localStorage.setItem(rightRailCollapsedStorageKey, rightRailCollapsed ? "1" : "0");
    } catch {
      // no-op
    }
  }, [rightRailCollapsed]);

  const overviewQuery = useQuery({
    queryKey: ["overview"],
    queryFn: dashboardApi.overview,
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

  const isGroupActive = (group: NavGroup) => {
    if (group.type === "link") {
      if (group.end) {
        return location.pathname === group.to;
      }
      return location.pathname === group.to || location.pathname.startsWith(group.to + "/");
    }
    return group.items.some((item) => {
      if (item.end) {
        return location.pathname === item.to;
      }
      return location.pathname === item.to || location.pathname.startsWith(item.to + "/");
    });
  };

  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(() => {
    const initial = new Set<string>();
    navGroups.forEach((group) => {
      if (isGroupActive(group)) {
        initial.add(group.key);
      }
    });
    return initial;
  });

  useEffect(() => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      navGroups.forEach((group) => {
        if (isGroupActive(group)) {
          next.add(group.key);
        }
      });
      return next;
    });
  }, [location.pathname]);

  const toggleGroup = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
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
            {navGroups.map((group) => {
              const Icon = group.icon;
              const groupActive = isGroupActive(group);
              const isExpanded = expandedGroups.has(group.key);

              return (
                <div key={group.key} className={styles.navGroup}>
                  {group.type === "link" ? (
                    <NavLink
                      end={group.end}
                      to={group.to}
                      className={({ isActive }) =>
                        `${styles.navGroupHeader} ${isActive ? styles.navGroupHeaderActive : ""}`
                      }
                      onClick={closeDrawers}
                    >
                      <Icon size={16} className={styles.navGroupIcon} />
                      <span className={styles.navGroupLabel}>{group.label}</span>
                    </NavLink>
                  ) : (
                    <>
                      <button
                        type="button"
                        className={`${styles.navGroupHeader} ${groupActive ? styles.navGroupHeaderActive : ""}`}
                        onClick={() => toggleGroup(group.key)}
                      >
                        <Icon size={16} className={styles.navGroupIcon} />
                        <span className={styles.navGroupLabel}>{group.label}</span>
                        {isExpanded ? (
                          <ChevronDown size={14} className={styles.navGroupChevron} />
                        ) : (
                          <ChevronRight size={14} className={styles.navGroupChevron} />
                        )}
                      </button>
                      <div
                        className={`${styles.navGroupItems} ${
                          isExpanded ? styles.navGroupItemsExpanded : styles.navGroupItemsCollapsed
                        }`}
                      >
                        {group.items.map((item) => (
                          <NavLink
                            key={item.to}
                            end={item.end}
                            to={item.to}
                            className={({ isActive }) =>
                              isActive ? styles.navSubActive : styles.navSubLink
                            }
                            onClick={closeDrawers}
                          >
                            {item.label}
                          </NavLink>
                        ))}
                      </div>
                    </>
                  )}
                </div>
              );
            })}
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
