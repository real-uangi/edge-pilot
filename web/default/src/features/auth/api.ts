import { request } from "../../shared/lib/api-client";
import type { SessionInfo, LoginInput } from "../../shared/types";

export const authApi = {
  login(input: LoginInput) {
    return request<SessionInfo>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  logout() {
    return request<{ ok: boolean }>("/api/auth/logout", {
      method: "POST",
    });
  },
  me() {
    return request<SessionInfo>("/api/auth/me");
  },
};
