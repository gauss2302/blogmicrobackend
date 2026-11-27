export const AUTH_REFRESH_COOKIE_NAME =
  process.env.AUTH_REFRESH_COOKIE_NAME || "refresh_token";

export const BACKEND_API_URL =
  process.env.BACKEND_API_URL || "http://localhost:8080";

// How many trusted reverse proxies sit in front of this server (0 = none, the
// browser connects directly). It decides whether X-Forwarded-For can be believed
// when reporting the client address to the gateway; see forwardedClientIp.
export const TRUSTED_PROXY_HOPS = Math.max(
  0,
  Number.parseInt(process.env.BFF_TRUSTED_PROXY_HOPS ?? "", 10) || 0,
);
