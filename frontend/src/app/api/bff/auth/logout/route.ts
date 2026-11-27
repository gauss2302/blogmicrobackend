import { isCrossSiteRequest } from "@/lib/server/csrf";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader, clearSessionCookies, readRefreshToken } from "@/lib/server/session";

export async function POST(request: Request) {
  if (isCrossSiteRequest(request)) {
    return toFailureResponse(403, "CROSS_ORIGIN_FORBIDDEN", "Cross-origin request rejected.");
  }

  // The access cookie may have lapsed while the refresh token still lives:
  // the gateway revokes by either, so call it whenever one is present.
  const authorization = authorizationHeader(request);
  const refreshToken = readRefreshToken(request);
  if (!authorization && !refreshToken) {
    // No session cookie: nothing to revoke upstream, but still clear anything
    // stale in the browser so the client lands in a clean logged-out state.
    const response = toSuccessResponse({ loggedOut: true }, 200);
    clearSessionCookies(response);
    return response;
  }

  const { upstream, payload, setCookies } = await proxyGateway<{ loggedOut?: boolean }>(
    request,
    "/api/v1/auth/logout",
    {
      method: "POST",
      headers: authorization ? { authorization } : {},
      body: JSON.stringify({ refresh_token: refreshToken ?? undefined }),
    },
  );

  // Even when upstream revocation fails (including a 429 from the auth rate
  // limiter, where the gateway clears nothing), drop the local cookies: leaving
  // a credential in the browser after the user asked to log out is the worse
  // outcome. The upstream error is still surfaced.
  if (!payload?.success) {
    const response = toFailureResponse(
      upstream.status,
      "LOGOUT_FAILED",
      "Logout failed.",
      payload?.error,
    );
    clearSessionCookies(response);
    return response;
  }

  const response = toSuccessResponse({ loggedOut: true }, upstream.status, setCookies);
  clearSessionCookies(response);
  return response;
}
