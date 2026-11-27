import type { NextResponse } from "next/server";

import { AUTH_REFRESH_COOKIE_NAME } from "@/lib/auth/server-constants";
import { mapSessionPayload, type BackendAuthPayload } from "@/lib/server/auth-mapper";
import { toSuccessResponse } from "@/lib/server/gateway";

/**
 * Server-side session handling for the BFF.
 *
 * The access token never reaches the browser. It is stored in an HttpOnly
 * cookie that the BFF sets, and every protected BFF route reads it back out and
 * injects it as `Authorization: Bearer` on the upstream gateway call.
 *
 * Why this matters: previously the access token was returned in the JSON body,
 * kept in a Zustand store and sent from JS as a Bearer header. That gave the
 * cost of a BFF (an extra hop, one route handler per endpoint) without its
 * defining benefit — an XSS payload could read the token straight out of the
 * store and exfiltrate it to an attacker-controlled host, where it stays valid
 * until expiry. With the token in an HttpOnly cookie, script running on the
 * page can still *call* same-origin BFF routes, but it cannot read the
 * credential or replay it anywhere else.
 *
 * The refresh token is an HttpOnly cookie too: issued by the gateway and
 * re-hardened in `gateway.ts` in cookie mode, or set here from the JSON body
 * when the gateway runs without it.
 */

export const AUTH_ACCESS_COOKIE_NAME =
  process.env.AUTH_ACCESS_COOKIE_NAME || "access_token";

/**
 * Skew subtracted from the upstream `expires_in` when setting cookie Max-Age,
 * so the cookie lapses slightly before the token does. That way a request never
 * carries a token the gateway has already rejected as expired — the client sees
 * "no session" and refreshes instead of eating a 401 mid-action.
 */
const ACCESS_COOKIE_SKEW_SECONDS = 15;

function cookieOptions(maxAge?: number) {
  return {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "strict" as const,
    path: "/",
    ...(maxAge !== undefined ? { maxAge } : {}),
  };
}

/** Stores the upstream access token as an HttpOnly cookie on the response. */
export function setAccessCookie(
  response: NextResponse,
  accessToken: string,
  expiresInSeconds: number,
) {
  const maxAge = Math.max(1, Math.floor(expiresInSeconds) - ACCESS_COOKIE_SKEW_SECONDS);
  response.cookies.set(AUTH_ACCESS_COOKIE_NAME, accessToken, cookieOptions(maxAge));
}

/**
 * Non-secret "probably signed in" marker for server-side redirects (proxy, root
 * page). The session cookies are SameSite=Strict, so a top-level visit from
 * another site arrives without them; this one is Lax and does arrive. It grants
 * nothing: a stale hint only lets the client-side session check run, which then
 * sends the user to the login page.
 */
export const SESSION_HINT_COOKIE_NAME = "session_hint";
const SESSION_HINT_MAX_AGE_SECONDS = 30 * 24 * 3600;

function hintCookieOptions(maxAge: number) {
  return { ...cookieOptions(maxAge), sameSite: "lax" as const };
}

/**
 * Whether a request may carry a session. The refresh cookie is the fallback
 * for sessions minted before the hint existed (same-site navigations only).
 */
export function mayHaveSession(cookies: {
  get(name: string): { value: string } | undefined;
}): boolean {
  return Boolean(
    cookies.get(SESSION_HINT_COOKIE_NAME)?.value ||
      cookies.get(AUTH_REFRESH_COOKIE_NAME)?.value,
  );
}

/** Clears the session cookies and the hint. Used by logout and by failed refresh. */
export function clearSessionCookies(response: NextResponse) {
  response.cookies.set(AUTH_ACCESS_COOKIE_NAME, "", cookieOptions(0));
  response.cookies.set(AUTH_REFRESH_COOKIE_NAME, "", cookieOptions(0));
  response.cookies.set(SESSION_HINT_COOKIE_NAME, "", hintCookieOptions(0));
}

/**
 * Reads the access token from the request cookie.
 *
 * Parsed by hand rather than via `next/headers` so this works uniformly in
 * route handlers that receive a plain `Request`.
 */
export function readAccessToken(request: Request): string | null {
  return readCookie(request, AUTH_ACCESS_COOKIE_NAME);
}

/** Reads the refresh token cookie, whichever side (gateway or BFF) set it. */
export function readRefreshToken(request: Request): string | null {
  return readCookie(request, AUTH_REFRESH_COOKIE_NAME);
}

function readCookie(request: Request, name: string): string | null {
  const header = request.headers.get("cookie");
  if (!header) return null;

  for (const part of header.split(";")) {
    const eq = part.indexOf("=");
    if (eq < 0) continue;
    if (part.slice(0, eq).trim() !== name) continue;
    const value = part.slice(eq + 1).trim();
    return value ? decodeURIComponent(value) : null;
  }
  return null;
}

/**
 * Single exit point for the four routes that mint a session (login, register,
 * OAuth exchange, refresh): returns the browser-safe payload and stores the
 * access token in its HttpOnly cookie in one step, so no route can accidentally
 * leak the token by forgetting to strip it. Also (re)sets the session hint.
 */
export function toSessionResponse(
  data: BackendAuthPayload,
  status: number,
  setCookies?: string[] | null,
) {
  const response = toSuccessResponse(mapSessionPayload(data), status, setCookies);
  setAccessCookie(response, data.access_token, data.expires_in);
  // Without AUTH_REFRESH_TOKEN_COOKIE the gateway returns the refresh token in
  // JSON instead of a Set-Cookie; keep it in the same HttpOnly cookie.
  if (data.refresh_token) {
    const ttl = accessTokenExpiresIn(data.refresh_token);
    response.cookies.set(AUTH_REFRESH_COOKIE_NAME, data.refresh_token, cookieOptions(ttl > 0 ? ttl : undefined));
  }
  response.cookies.set(SESSION_HINT_COOKIE_NAME, "1", hintCookieOptions(SESSION_HINT_MAX_AGE_SECONDS));
  return response;
}

/**
 * Builds the upstream Authorization header, or null when there is no session.
 *
 * Protected routes call this instead of trusting a client-supplied header — the
 * browser cannot forge a session it does not hold, because the cookie is
 * HttpOnly and the value is never exposed to script.
 */
export function authorizationHeader(request: Request): string | null {
  const token = readAccessToken(request);
  return token ? `Bearer ${token}` : null;
}

/**
 * Seconds until a JWT's `exp` claim (access or refresh token). The signature is not checked
 * here — callers must already have validated the token. A missing or past `exp`
 * returns 0 so the client does not invent a lifetime.
 */
export function accessTokenExpiresIn(
  token: string,
  nowSeconds = Math.floor(Date.now() / 1000),
): number {
  const payload = token.split(".")[1];
  if (!payload) return 0;
  try {
    const json = JSON.parse(
      Buffer.from(payload, "base64url").toString("utf8"),
    ) as { exp?: unknown };
    if (typeof json.exp !== "number" || !Number.isFinite(json.exp)) return 0;
    return Math.max(0, Math.floor(json.exp) - nowSeconds);
  } catch {
    return 0;
  }
}
