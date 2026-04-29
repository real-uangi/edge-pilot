export interface ApiEnvelope<T> {
  code: number;
  data: T;
  message: string;
  time: string;
}

export interface SessionInfo {
  username: string;
  expiresAt: string;
}

export interface LoginInput {
  username: string;
  password: string;
}
