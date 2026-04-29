export interface RegistryCredentialRecord {
  id: string;
  registryHost: string;
  username: string;
  secretConfigured: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface UpsertRegistryCredentialInput {
  registryHost: string;
  username: string;
  secret: string;
}
