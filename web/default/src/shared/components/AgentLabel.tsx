import { formatAgentLabel } from "../lib/format";

type AgentLabelProps = {
  id: string;
  hostname?: string | null;
  ip?: string | null;
};

export function AgentLabel({ id, hostname, ip }: AgentLabelProps) {
  return <>{formatAgentLabel({ id, hostname, ip })}</>;
}
