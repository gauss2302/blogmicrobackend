import { isCrossSiteRequest } from "@/lib/server/csrf";
import { mapSessionPayload, type BackendAuthPayload } from "@/lib/server/auth-mapper";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function POST(request: Request) {
  // Cookie-authenticated: reject cross-site callers to prevent CSRF.
  if (isCrossSiteRequest(request)) {
    return toFailureResponse(403, "CROSS_ORIGIN_FORBIDDEN", "Cross-origin request rejected.");
  }

  const { upstream, payload, setCookie } = await proxyGateway<BackendAuthPayload>(
    request,
    "/api/v1/auth/refresh",
    {
      method: "POST",
      body: JSON.stringify({}),
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "REFRESH_FAILED",
      "Token refresh failed.",
      payload?.error,
    );
  }

  return toSuccessResponse(mapSessionPayload(payload.data), upstream.status, setCookie);
}
