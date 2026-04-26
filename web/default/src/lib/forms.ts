import { z } from "zod";
import type { ServiceRecord, UpsertServiceInput } from "./api";

const lineList = (value: string) =>
  value
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean);

function parseEnv(text: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of lineList(text)) {
    const index = line.indexOf("=");
    if (index <= 0) {
      throw new Error(`Invalid env line: ${line}`);
    }
    result[line.slice(0, index).trim()] = line.slice(index + 1).trim();
  }
  return result;
}

function parseHeaders(text: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of lineList(text)) {
    const index = line.indexOf(":");
    if (index <= 0) {
      throw new Error(`Invalid header line: ${line}`);
    }
    const key = line.slice(0, index).trim();
    const value = line.slice(index + 1).trim();
    if (!key || !value) {
      throw new Error(`Invalid header line: ${line}`);
    }
    result[key] = value;
  }
  return result;
}

function parseVolumes(text: string): Array<{ source: string; target: string; readOnly: boolean }> {
  return lineList(text).map((line) => {
    const parts = line.split(":").map((item) => item.trim());
    if (parts.length < 2 || parts.length > 3) {
      throw new Error(`Invalid volume line: ${line}`);
    }
    return {
      source: parts[0],
      target: parts[1],
      readOnly: parts[2] === "ro",
    };
  });
}

function parsePublishedPorts(text: string): Array<{ hostPort: number; containerPort: number }> {
 return lineList(text).map((line) => {
    const parts = line.split(":").map((item) => item.trim());
    if (parts.length !== 2) {
      throw new Error(`Invalid published port line: ${line}`);
    }
    return {
      hostPort: Number(parts[0]),
      containerPort: Number(parts[1]),
    };
  });
}

function parseNetworkAliases(text: string): string[] {
  return lineList(text);
}

export const serviceFormSchema = z.object({
  name: z.string().min(1, "必填"),
  serviceKey: z.string().min(1, "必填"),
  agentId: z.string().uuid("必须是 UUID"),
  imageRepo: z.string().min(1, "必填"),
  containerPort: z.coerce.number().int().positive(),
  cpuLimitCores: z.coerce.number().min(0),
  memoryLimitMB: z.coerce.number().int().min(0),
  dockerHealthCheck: z.boolean(),
  httpHealthPath: z.string().trim(),
  httpExpectedCode: z.coerce.number().int().positive(),
  httpTimeoutSecond: z.coerce.number().int().positive(),
  startupGraceSecond: z.coerce.number().int().min(0),
  httpProbeTimeoutSecond: z.coerce.number().int().positive(),
  httpProbeIntervalSecond: z.coerce.number().int().positive(),
  httpSuccessThreshold: z.coerce.number().int().positive(),
  schedulerSdkPort: z.coerce.number().int().min(0).max(65535),
  schedulerExecutorGroup: z.string().trim(),
  routeHost: z.string().min(1, "必填"),
  routePathPrefix: z.string().trim(),
  enabled: z.boolean(),
  httpHealthHeadersText: z.string(),
  envText: z.string(),
  commandText: z.string(),
  entrypointText: z.string(),
  volumesText: z.string(),
  networkAliasesText: z.string(),
  publishedPortsText: z.string(),
});

export type ServiceFormInput = z.input<typeof serviceFormSchema>;
export type ServiceFormValues = z.output<typeof serviceFormSchema>;

export function toServicePayload(values: ServiceFormValues): UpsertServiceInput {
  return {
    name: values.name.trim(),
    serviceKey: values.serviceKey.trim(),
    agentId: values.agentId.trim(),
    imageRepo: values.imageRepo.trim(),
    containerPort: values.containerPort,
    cpuLimitCores: values.cpuLimitCores,
    memoryLimitMB: values.memoryLimitMB,
    dockerHealthCheck: values.dockerHealthCheck,
    httpHealthPath: values.httpHealthPath.trim(),
    httpHealthHeaders: parseHeaders(values.httpHealthHeadersText),
    httpExpectedCode: values.httpExpectedCode,
    httpTimeoutSecond: values.httpTimeoutSecond,
    startupGraceSecond: values.startupGraceSecond,
    httpProbeTimeoutSecond: values.httpProbeTimeoutSecond,
    httpProbeIntervalSecond: values.httpProbeIntervalSecond,
    httpSuccessThreshold: values.httpSuccessThreshold,
    schedulerSdkPort: values.schedulerSdkPort,
    schedulerExecutorGroup: values.schedulerSdkPort > 0 && !values.schedulerExecutorGroup.trim() ? "default" : values.schedulerExecutorGroup.trim(),
    routeHost: values.routeHost.trim(),
    routePathPrefix: values.routePathPrefix.trim(),
    enabled: values.enabled,
    env: parseEnv(values.envText),
    command: lineList(values.commandText),
    entrypoint: lineList(values.entrypointText),
    volumes: parseVolumes(values.volumesText),
    networkAliases: parseNetworkAliases(values.networkAliasesText),
    publishedPorts: parsePublishedPorts(values.publishedPortsText),
  };
}

export function toServiceFormDefaults(service?: ServiceRecord): ServiceFormInput {
  return {
    name: service?.name ?? "",
    serviceKey: service?.serviceKey ?? "",
    agentId: service?.agentId ?? "",
    imageRepo: service?.imageRepo ?? "",
    containerPort: service?.containerPort ?? 8080,
    cpuLimitCores: service?.cpuLimitCores ?? 0,
    memoryLimitMB: service?.memoryLimitMB ?? 0,
    dockerHealthCheck: service?.dockerHealthCheck ?? true,
    httpHealthPath: service?.httpHealthPath ?? "",
    httpExpectedCode: service?.httpExpectedCode ?? 200,
    httpTimeoutSecond: service?.httpTimeoutSecond ?? 90,
    startupGraceSecond: service?.startupGraceSecond ?? 15,
    httpProbeTimeoutSecond: service?.httpProbeTimeoutSecond ?? 2,
    httpProbeIntervalSecond: service?.httpProbeIntervalSecond ?? 1,
    httpSuccessThreshold: service?.httpSuccessThreshold ?? 2,
    schedulerSdkPort: service?.schedulerSdkPort ?? 0,
    schedulerExecutorGroup: service?.schedulerExecutorGroup ?? "default",
    routeHost: service?.routeHost ?? "",
    routePathPrefix: service?.routePathPrefix ?? "/",
    enabled: service?.enabled ?? true,
    httpHealthHeadersText: Object.entries(service?.httpHealthHeaders ?? {})
      .map(([key, value]) => `${key}: ${value}`)
      .join("\n"),
    envText: Object.entries(service?.env ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join("\n"),
    commandText: (service?.command ?? []).join("\n"),
    entrypointText: (service?.entrypoint ?? []).join("\n"),
    volumesText: (service?.volumes ?? [])
      .map((item) => `${item.source}:${item.target}${item.readOnly ? ":ro" : ""}`)
      .join("\n"),
    networkAliasesText: (service?.networkAliases ?? []).join("\n"),
    publishedPortsText: (service?.publishedPorts ?? [])
      .map((item) => `${item.hostPort}:${item.containerPort}`)
      .join("\n"),
  };
}
