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

/**
 * Maps the upstream auth payload to what the browser is allowed to see.
 *
 * Deliberately omits `access_token`: it goes into an HttpOnly cookie via
 * `setAccessCookie` instead. `expiresIn` is still returned so the client knows
 * when to schedule a refresh, but it is not a credential.
 */
export function mapSessionPayload(data: BackendAuthPayload): SessionPayload {
  return {
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
