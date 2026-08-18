export type APIErrorCode =
  | "INVALID_REQUEST"
  | "UNAUTHORIZED"
  | "LOGIN_FAILED"
  | "REGISTER_FAILED"
  | "EXCHANGE_FAILED"
  | "REFRESH_FAILED"
  | "CALLBACK_FAILED"
  | "AUTH_URL_FAILED"
  | "INTERNAL_ERROR";

export interface APIError {
  code: APIErrorCode | string;
  message: string;
}

export interface APISuccess<T> {
  success: true;
  data: T;
}

export interface APIFailure {
  success: false;
  error: APIError;
}

export type APIEnvelope<T> = APISuccess<T> | APIFailure;

export interface UserInfo {
  id: string;
  email: string;
  name?: string;
  picture?: string;
}

/**
 * What the browser learns about its session.
 *
 * There is deliberately no `accessToken` here — the credential lives in an
 * HttpOnly cookie the BFF manages, so script cannot read or exfiltrate it.
 * `expiresIn` is kept only so the client can schedule a refresh.
 */
export interface SessionPayload {
  tokenType: string;
  expiresIn: number;
  user: UserInfo;
}

export interface GoogleAuthURLPayload {
  authUrl: string;
  state: string;
}
