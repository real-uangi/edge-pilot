import { request } from "../../shared/lib/api-client";
import type { OverviewRecord } from "./types";

export const dashboardApi = {
  overview() {
    return request<OverviewRecord>("/api/admin/overview");
  },
};
