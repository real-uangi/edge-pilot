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

export const serviceFormSchema = z.object({
  name: z.string().min(1, "必填"),
  serviceKey: z.string().min(1, "必填"),
  agentId: z.string().uuid("必须是 UUID"),
  imageRepo: z.string().min(1, "必填"),
  containerPort: z.coerce.number().int().positive(),
  dockerHealthCheck: z.boolean(),
  httpHealthPath: z.string().trim(),
  httpExpectedCode: z.coerce.number().int().positive(),
  httpTimeoutSecond: z.coerce.number().int().positive(),
  routeHost: z.string().min(1, "必填"),
  routePathPrefix: z.string().trim(),
  enabled: z.boolean(),
  envText: z.string(),
  commandText: z.string(),
  entrypointText: z.string(),
  volumesText: z.string(),
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
    dockerHealthCheck: values.dockerHealthCheck,
    httpHealthPath: values.httpHealthPath.trim(),
    httpExpectedCode: values.httpExpectedCode,
    httpTimeoutSecond: values.httpTimeoutSecond,
    routeHost: values.routeHost.trim(),
    routePathPrefix: values.routePathPrefix.trim(),
    enabled: values.enabled,
    env: parseEnv(values.envText),
    command: lineList(values.commandText),
    entrypoint: lineList(values.entrypointText),
    volumes: parseVolumes(values.volumesText),
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
    dockerHealthCheck: service?.dockerHealthCheck ?? true,
    httpHealthPath: service?.httpHealthPath ?? "",
    httpExpectedCode: service?.httpExpectedCode ?? 200,
    httpTimeoutSecond: service?.httpTimeoutSecond ?? 5,
    routeHost: service?.routeHost ?? "",
    routePathPrefix: service?.routePathPrefix ?? "/",
    enabled: service?.enabled ?? true,
    envText: Object.entries(service?.env ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join("\n"),
    commandText: (service?.command ?? []).join("\n"),
    entrypointText: (service?.entrypoint ?? []).join("\n"),
    volumesText: (service?.volumes ?? [])
      .map((item) => `${item.source}:${item.target}${item.readOnly ? ":ro" : ""}`)
      .join("\n"),
    publishedPortsText: (service?.publishedPorts ?? [])
      .map((item) => `${item.hostPort}:${item.containerPort}`)
      .join("\n"),
  };
}
