import {
  mapGoogleAuthURLPayload,
  type BackendGoogleAuthURLPayload,
} from "@/lib/server/auth-mapper";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function GET(request: Request) {
  const requestURL = new URL(request.url);
  const query = requestURL.searchParams.toString();
  const path = query
    ? `/api/v1/auth/google?${query}`
    : "/api/v1/auth/google";

  const { upstream, payload } = await proxyGateway<BackendGoogleAuthURLPayload>(
    request,
    path,
    {
      method: "GET",
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "AUTH_URL_FAILED",
      "Failed to get Google OAuth URL.",
      payload?.error,
    );
  }

  return toSuccessResponse(mapGoogleAuthURLPayload(payload.data), upstream.status);
}
