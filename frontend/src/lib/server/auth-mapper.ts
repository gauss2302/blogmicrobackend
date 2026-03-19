import type { SessionPayload, UserInfo } from "@/lib/auth/types";

export interface BackendAuthPayload {
  access_token: string;
  refresh_token?: string;
  token_type: string;
  expires_in: number;
  user: UserInfo;
}

export interface BackendGoogleAuthURLPayload {
  auth_url: string;
  state: string;
}

export function mapSessionPayload(data: BackendAuthPayload): SessionPayload {
  return {
    accessToken: data.access_token,
    tokenType: data.token_type,
    expiresIn: data.expires_in,
    user: data.user,
  };
}

export function mapGoogleAuthURLPayload(data: BackendGoogleAuthURLPayload) {
  return {
    authUrl: data.auth_url,
    state: data.state,
  };
}
