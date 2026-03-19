import { mapSessionPayload, type BackendAuthPayload } from "@/lib/server/auth-mapper";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function POST(request: Request) {
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
