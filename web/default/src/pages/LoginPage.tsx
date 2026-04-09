import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocation, useNavigate } from "react-router-dom";
import { api, getErrorMessage } from "../lib/api";
import styles from "./LoginPage.module.css";

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const loginMutation = useMutation({
    mutationFn: api.login,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["session"] });
      navigate(location.state?.from ?? "/", { replace: true });
    },
  });

  return (
    <div className={styles.page}>
      <section className={styles.panel}>
        <div className={styles.copy}>
          <span className={styles.eyebrow}>Edge Pilot</span>
          <h1 className={styles.title}>Sign In To The Control Plane</h1>
          <p className={styles.subtitle}>管理操作、发布动作和运行态视图都通过这一张入口页收口。</p>
        </div>
        <form
          className={styles.form}
          onSubmit={(event) => {
            event.preventDefault();
            loginMutation.mutate({ username, password });
          }}
        >
          <label className={styles.field}>
            <span>Username</span>
            <input value={username} onChange={(event) => setUsername(event.target.value)} />
          </label>
          <label className={styles.field}>
            <span>Password</span>
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
          {loginMutation.isError ? (
            <div className={styles.error}>{getErrorMessage(loginMutation.error)}</div>
          ) : null}
          <button className={styles.submit} disabled={loginMutation.isPending} type="submit">
            {loginMutation.isPending ? "Signing In" : "Sign In"}
          </button>
        </form>
      </section>
    </div>
  );
}
