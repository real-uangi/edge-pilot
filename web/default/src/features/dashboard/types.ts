import type { AgentOverview } from "../agents/types";
import type { ServiceRecord } from "../services/types";
import type { ReleaseRecord } from "../releases/types";

export interface OverviewRecord {
  agents: AgentOverview[];
  services: ServiceRecord[];
  recentReleases: ReleaseRecord[];
  activeInstances: number;
}
