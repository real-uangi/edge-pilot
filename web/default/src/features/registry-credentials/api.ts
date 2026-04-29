import { request } from "../../shared/lib/api-client";
import type { RegistryCredentialRecord, UpsertRegistryCredentialInput } from "./types";

export const registryCredentialsApi = {
  list() {
    return request<RegistryCredentialRecord[]>("/api/admin/registry-credentials");
  },
  get(id: string) {
    return request<RegistryCredentialRecord>(`/api/admin/registry-credentials/${id}`);
  },
  create(input: UpsertRegistryCredentialInput) {
    return request<RegistryCredentialRecord>("/api/admin/registry-credentials", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  update(id: string, input: UpsertRegistryCredentialInput) {
    return request<RegistryCredentialRecord>(`/api/admin/registry-credentials/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    });
  },
  delete(id: string) {
    return request<{ deleted: boolean }>(`/api/admin/registry-credentials/${id}`, {
      method: "DELETE",
    });
  },
};
