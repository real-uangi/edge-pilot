import type { PropsWithChildren } from "react";
import { NavLink } from "react-router-dom";
import styles from "./AppShell.module.css";

const navItems = [
  { to: "/", label: "Overview", end: true },
  { to: "/services", label: "Services" },
  { to: "/agents", label: "Agents" },
  { to: "/releases", label: "Releases" },
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
            <span className={styles.meta}>Control Plane</span>
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
              {loggingOut ? "Signing Out" : "Sign Out"}
            </button>
          </div>
        </div>
      </header>
      <main className={styles.main}>{children}</main>
    </div>
  );
}
