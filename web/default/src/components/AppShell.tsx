import type { PropsWithChildren } from "react";
import { NavLink } from "react-router-dom";
import styles from "./AppShell.module.css";

const navItems = [
  { to: "/", label: "总览", end: true },
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

export function AppShell({ username, loggingOut, onLogout, children }: AppShellProps) {
  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <div className={styles.headerInner}>
          <div className={styles.brandBlock}>
            <span className={styles.brand}>Edge Pilot</span>
            <span className={styles.meta}>管理面板</span>
          </div>
          <nav className={styles.nav}>
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                end={item.end}
                to={item.to}
                className={({ isActive }) => (isActive ? styles.navActive : styles.navLink)}
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className={styles.actions}>
            <div className={styles.user}>{username}</div>
            <button className={styles.logout} disabled={loggingOut} onClick={onLogout} type="button">
              {loggingOut ? "退出中" : "退出登录"}
            </button>
          </div>
        </div>
      </header>
      <main className={styles.main}>{children}</main>
    </div>
  );
}
