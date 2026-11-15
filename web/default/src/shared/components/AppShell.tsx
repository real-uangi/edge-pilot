import { useEffect, useRef, useState, type PropsWithChildren } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, NavLink, useLocation } from "react-router-dom";
import {
  Activity,
  ArrowUpRight,
  Boxes,
  CalendarClock,
  ChevronRight,
  History,
  KeyRound,
  LayoutDashboard,
  ListChecks,
  LogOut,
  Menu,
  Rocket,
  Server,
  Waypoints,
  type LucideIcon,
} from "lucide-react";
import { dashboardApi } from "../../features/dashboard/api";
import {
  formatDateTime,
  releaseStatusLabel,
  releaseStatusTone,
  shortId,
} from "../lib/format";
import { ActionButton } from "./ActionButton";
import { Modal } from "./Modal";
import { ErrorState, InlineNotice, LoadingState } from "./StateBlocks";
import { StatusPill } from "./StatusPill";
import styles from "./AppShell.module.css";

type NavItem = { to: string; label: string; icon: LucideIcon; end?: boolean };

const navGroups: Array<{ label: string; items: NavItem[] }> = [
  {
    label: "工作台",
    items: [{ to: "/", label: "总览", icon: LayoutDashboard, end: true }],
  },
  {
    label: "应用与发布",
    items: [
      { to: "/services", label: "服务", icon: Boxes },
      { to: "/releases", label: "发布记录", icon: Rocket },
      { to: "/instances", label: "受管实例", icon: Waypoints },
    ],
  },
  {
    label: "基础设施",
    items: [
      { to: "/agents", label: "节点", icon: Server },
      { to: "/registry-credentials", label: "镜像仓库", icon: KeyRound },
    ],
  },
  {
    label: "任务调度",
    items: [
      { to: "/scheduler", label: "定时任务", icon: CalendarClock, end: true },
      { to: "/scheduler/history", label: "执行历史", icon: History },
      { to: "/scheduler/executors", label: "执行器", icon: ListChecks },
    ],
  },
  {
    label: "系统",
    items: [{ to: "/system-performance", label: "系统性能", icon: Activity }],
  },
];

interface AppShellProps extends PropsWithChildren {
  username: string;
  loggingOut: boolean;
  onLogout: () => void;
}

export function AppShell({
  username,
  loggingOut,
  onLogout,
  children,
}: AppShellProps) {
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [healthOpen, setHealthOpen] = useState(false);
  const mainRef = useRef<HTMLElement>(null);
  const location = useLocation();
  const activeItem = navGroups
    .flatMap((group) => group.items)
    .find((item) =>
      item.end
        ? location.pathname === item.to
        : location.pathname === item.to ||
          location.pathname.startsWith(`${item.to}/`),
    );
  const isDetail = activeItem && location.pathname !== activeItem.to;
  const pageTitle =
    location.pathname === "/services/new"
      ? "新建服务"
      : isDetail
        ? `${activeItem.label === "发布记录" ? "发布" : activeItem.label}详情`
        : (activeItem?.label ?? "页面未找到");

  useEffect(() => {
    setNavigationOpen(false);
    setHealthOpen(false);
    document.title = `${pageTitle} · Edge Pilot`;
    const frame = requestAnimationFrame(() =>
      mainRef.current?.focus({ preventScroll: true }),
    );
    return () => cancelAnimationFrame(frame);
  }, [location.pathname, pageTitle]);

  useEffect(() => {
    const desktop = window.matchMedia("(min-width: 1200px)");
    const closeDesktopNavigation = () => {
      if (desktop.matches) setNavigationOpen(false);
    };
    desktop.addEventListener("change", closeDesktopNavigation);
    return () => desktop.removeEventListener("change", closeDesktopNavigation);
  }, []);

  const overviewQuery = useQuery({
    queryKey: ["overview"],
    queryFn: dashboardApi.overview,
    enabled: healthOpen,
    refetchInterval: healthOpen ? 10000 : false,
  });
  const overview = overviewQuery.data;

  const navigation = (
    <>
      <nav className={styles.nav} aria-label="主导航">
        {navGroups.map((group) => (
          <div key={group.label} className={styles.navGroup}>
            <span className={styles.groupLabel}>{group.label}</span>
            {group.items.map((item) => {
              const Icon = item.icon;
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) =>
                    `${styles.navLink} ${isActive ? styles.navActive : ""}`
                  }
                  onClick={() => setNavigationOpen(false)}
                >
                  <Icon size={17} strokeWidth={1.7} aria-hidden="true" />
                  <span>{item.label}</span>
                </NavLink>
              );
            })}
          </div>
        ))}
      </nav>
      <div className={styles.session}>
        <span className={styles.avatar} aria-hidden="true">
          {username.slice(0, 1).toUpperCase()}
        </span>
        <div className={styles.sessionInfo}>
          <span className={styles.username}>{username}</span>
          <span className={styles.sessionLabel}>管理员</span>
        </div>
        <button
          type="button"
          className={styles.iconButton}
          disabled={loggingOut}
          onClick={onLogout}
          aria-label={loggingOut ? "正在退出登录" : "退出登录"}
          title="退出登录"
        >
          <LogOut size={17} aria-hidden="true" />
        </button>
      </div>
    </>
  );

  return (
    <div className={styles.shell}>
      <a href="#main-content" className={styles.skipLink}>
        跳到主内容
      </a>
      <aside className={styles.sidebar}>
        <Link to="/" className={styles.brand} aria-label="Edge Pilot 总览">
          <span className={styles.brandMark} aria-hidden="true">
            <Waypoints size={20} strokeWidth={1.8} />
          </span>
          <span>Edge Pilot</span>
        </Link>
        {navigation}
      </aside>
      <div className={styles.workspace}>
        <header className={styles.topbar}>
          <div className={styles.context}>
            <button
              type="button"
              className={`${styles.iconButton} ${styles.menuButton}`}
              aria-label="打开导航菜单"
              aria-haspopup="dialog"
              aria-expanded={navigationOpen}
              onClick={() => {
                setHealthOpen(false);
                setNavigationOpen(true);
              }}
            >
              <Menu size={20} aria-hidden="true" />
            </button>
            <nav className={styles.breadcrumb} aria-label="当前位置">
              <span className={styles.breadcrumbRoot}>工作台</span>
              <ChevronRight
                size={14}
                aria-hidden="true"
                className={styles.breadcrumbDivider}
              />
              {isDetail ? (
                <>
                  <Link to={activeItem.to}>{activeItem.label}</Link>
                  <ChevronRight
                    size={14}
                    aria-hidden="true"
                    className={styles.breadcrumbDivider}
                  />
                </>
              ) : null}
              <span className={styles.breadcrumbCurrent} aria-current="page">
                {pageTitle}
              </span>
            </nav>
          </div>
          <button
            type="button"
            className={styles.healthButton}
            aria-haspopup="dialog"
            aria-expanded={healthOpen}
            onClick={() => {
              setNavigationOpen(false);
              setHealthOpen(true);
            }}
          >
            <Activity size={16} strokeWidth={1.7} aria-hidden="true" />
            <span>系统状态</span>
          </button>
        </header>
        <main
          id="main-content"
          ref={mainRef}
          tabIndex={-1}
          className={styles.stage}
          aria-label={pageTitle}
        >
          <div className={styles.stageInner}>{children}</div>
        </main>
      </div>
      {navigationOpen ? (
        <Modal
          title="Edge Pilot"
          variant="drawer"
          side="left"
          onClose={() => setNavigationOpen(false)}
        >
          <div className={styles.drawerNavigation}>{navigation}</div>
        </Modal>
      ) : null}
      {healthOpen ? (
        <Modal
          title="系统状态"
          variant="drawer"
          onClose={() => setHealthOpen(false)}
        >
          <div className={styles.healthContent}>
            {!overview && overviewQuery.isPending ? (
              <LoadingState title="正在读取系统状态" />
            ) : null}
            {!overview && overviewQuery.isError ? (
              <ErrorState
                title="系统状态读取失败"
                message="请重试获取最新数据。"
                onRetry={() => {
                  void overviewQuery.refetch();
                }}
              />
            ) : null}
            {overview ? (
              <>
                <div className={styles.healthRefresh}>
                  <span>
                    {overviewQuery.isFetching
                      ? "正在刷新"
                      : `更新于 ${formatDateTime(new Date(overviewQuery.dataUpdatedAt).toISOString())}`}
                  </span>
                  <ActionButton
                    label="刷新"
                    pending={overviewQuery.isFetching}
                    onClick={() => {
                      void overviewQuery.refetch();
                    }}
                    variant="ghost"
                  />
                </div>
                {overviewQuery.isError ? (
                  <InlineNotice message="刷新失败，当前显示上次成功读取的数据。" />
                ) : null}
                <div className={styles.healthGrid}>
                  <Link
                    to="/agents"
                    onClick={() => setHealthOpen(false)}
                    className={styles.healthCard}
                  >
                    <span>在线节点</span>
                    <strong>
                      {overview.agents.filter((agent) => agent.online).length}
                      <small> / {overview.agents.length}</small>
                    </strong>
                  </Link>
                  <Link
                    to="/instances"
                    onClick={() => setHealthOpen(false)}
                    className={styles.healthCard}
                  >
                    <span>运行实例</span>
                    <strong>{overview.activeInstances}</strong>
                  </Link>
                  <Link
                    to="/releases"
                    onClick={() => setHealthOpen(false)}
                    className={styles.healthCard}
                  >
                    <span>近期活动发布</span>
                    <strong>
                      {
                        overview.recentReleases.filter(
                          (release) => release.isActive,
                        ).length
                      }
                    </strong>
                  </Link>
                  <Link
                    to="/releases"
                    onClick={() => setHealthOpen(false)}
                    className={styles.healthCard}
                  >
                    <span>近期失败发布</span>
                    <strong
                      className={
                        overview.recentReleases.some(
                          (release) => release.status === 7,
                        )
                          ? styles.failureValue
                          : undefined
                      }
                    >
                      {
                        overview.recentReleases.filter(
                          (release) => release.status === 7,
                        ).length
                      }
                    </strong>
                  </Link>
                </div>
                <section
                  className={styles.recentSection}
                  aria-labelledby="health-releases-title"
                >
                  <h3 id="health-releases-title">最近发布</h3>
                  {overview.recentReleases.length === 0 ? (
                    <p className={styles.healthEmpty}>暂无发布记录</p>
                  ) : (
                    <div className={styles.healthList}>
                      {overview.recentReleases.slice(0, 5).map((release) => (
                        <Link
                          to={`/releases/${release.id}`}
                          onClick={() => setHealthOpen(false)}
                          className={styles.healthItem}
                          key={release.id}
                        >
                          <div className={styles.healthItemInfo}>
                            <span className={styles.healthItemTitle}>
                              {release.imageTag}{" "}
                              <span className={styles.healthId}>
                                #{shortId(release.id)}
                              </span>
                            </span>
                            <span className={styles.healthRepository}>
                              {release.imageRepo}
                            </span>
                            <StatusPill
                              label={releaseStatusLabel(release.status)}
                              tone={releaseStatusTone(
                                release.status,
                                release.isActive,
                              )}
                            />
                          </div>
                          <ArrowUpRight size={16} aria-hidden="true" />
                        </Link>
                      ))}
                    </div>
                  )}
                </section>
              </>
            ) : null}
          </div>
        </Modal>
      ) : null}
    </div>
  );
}
