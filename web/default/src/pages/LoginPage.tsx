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
          <h1 className={styles.title}>登录管理面板</h1>
          <p className={styles.subtitle}>服务编排、节点控制、发布操作和运行态视图都从这里进入。</p>
        </div>
        <form
          className={styles.form}
          onSubmit={(event) => {
            event.preventDefault();
            loginMutation.mutate({ username, password });
          }}
        >
          <label className={styles.field}>
            <span>用户名</span>
            <input value={username} onChange={(event) => setUsername(event.target.value)} />
          </label>
          <label className={styles.field}>
            <span>密码</span>
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
            {loginMutation.isPending ? "登录中" : "登录"}
          </button>
        </form>
      </section>
    </div>
  );
}
