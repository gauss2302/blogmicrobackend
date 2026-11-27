import { isCrossSiteRequest } from "@/lib/server/csrf";
import { type BackendAuthPayload } from "@/lib/server/auth-mapper";
import { proxyGateway, toFailureResponse } from "@/lib/server/gateway";
import { clearSessionCookies, readRefreshToken, toSessionResponse } from "@/lib/server/session";

export async function POST(request: Request) {
  // Cookie-authenticated: reject cross-site callers to prevent CSRF.
  if (isCrossSiteRequest(request)) {
    return toFailureResponse(403, "CROSS_ORIGIN_FORBIDDEN", "Cross-origin request rejected.");
  }

  const { upstream, payload, setCookies } = await proxyGateway<BackendAuthPayload>(
    request,
    "/api/v1/auth/refresh",
    {
      method: "POST",
      // The gateway reads its own cookie first (cookie mode) and the body
      // otherwise, so sending both works in either mode.
      body: JSON.stringify({ refresh_token: readRefreshToken(request) ?? undefined }),
    },
  );

  if (!payload?.success || !payload.data) {
    const response = toFailureResponse(
      upstream.status,
      "REFRESH_FAILED",
      "Token refresh failed.",
      payload?.error,
    );
    // 401/400: the refresh token is dead or missing, so is the session. A 5xx
    // or 429 keeps the cookies for a retry, as the client does.
    if (upstream.status === 401 || upstream.status === 400) {
      clearSessionCookies(response);
    }
    return response;
  }

  return toSessionResponse(payload.data, upstream.status, setCookies);
}
