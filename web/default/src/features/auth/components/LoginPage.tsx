import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocation, useNavigate } from "react-router-dom";
import { Eye, EyeOff, Waypoints } from "lucide-react";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { authApi } from "../api";
import styles from "./LoginPage.module.css";

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [passwordVisible, setPasswordVisible] = useState(false);

  useEffect(() => {
    document.title = "登录 · Edge Pilot";
  }, []);

  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["session"] });
      navigate(location.state?.from ?? "/", { replace: true });
    },
  });

  return (
    <main className={styles.page}>
      <section className={styles.panel} aria-labelledby="login-title">
        <div className={styles.brand}>
          <span className={styles.brandMark} aria-hidden="true">
            <Waypoints size={23} strokeWidth={1.8} />
          </span>
          <span>Edge Pilot</span>
        </div>
        <h1 id="login-title" className={styles.title}>
          登录控制台
        </h1>
        <form
          className={styles.form}
          aria-busy={loginMutation.isPending}
          onSubmit={(event) => {
            event.preventDefault();
            if (!loginMutation.isPending)
              loginMutation.mutate({ username, password });
          }}
        >
          <label className={styles.field}>
            <span>用户名</span>
            <input
              name="username"
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              required
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              aria-describedby={
                loginMutation.isError ? "login-error" : undefined
              }
            />
          </label>
          <label className={styles.field}>
            <span>密码</span>
            <span className={styles.passwordField}>
              <input
                name="password"
                type={passwordVisible ? "text" : "password"}
                autoComplete="current-password"
                required
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                aria-describedby={
                  loginMutation.isError ? "login-error" : undefined
                }
              />
              <button
                type="button"
                className={styles.passwordToggle}
                aria-label={passwordVisible ? "隐藏密码" : "显示密码"}
                aria-pressed={passwordVisible}
                onClick={() => setPasswordVisible((visible) => !visible)}
              >
                {passwordVisible ? (
                  <EyeOff size={17} aria-hidden="true" />
                ) : (
                  <Eye size={17} aria-hidden="true" />
                )}
              </button>
            </span>
          </label>
          {loginMutation.isError ? (
            <div className={styles.error} id="login-error" role="alert">
              {getErrorMessage(loginMutation.error)}
            </div>
          ) : null}
          <button
            className={styles.submit}
            disabled={loginMutation.isPending}
            type="submit"
          >
            {loginMutation.isPending ? "正在登录" : "登录"}
          </button>
        </form>
      </section>
      <span className={styles.footer}>Edge Pilot Control Plane</span>
    </main>
  );
}
