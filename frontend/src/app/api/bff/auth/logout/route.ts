import { isCrossSiteRequest } from "@/lib/server/csrf";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader, clearSessionCookies } from "@/lib/server/session";

export async function POST(request: Request) {
  if (isCrossSiteRequest(request)) {
    return toFailureResponse(403, "CROSS_ORIGIN_FORBIDDEN", "Cross-origin request rejected.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
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
      headers: { authorization },
      body: JSON.stringify({}),
    },
  );

  // Even when upstream revocation fails, drop the local cookies: leaving a
  // credential in the browser after the user asked to log out is the worse
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
