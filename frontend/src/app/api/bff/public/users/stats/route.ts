import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

/** Public instance stats (active user count) — used on the marketing page. */
export async function GET(request: Request) {
  const { upstream, payload } = await proxyGateway<{ total_active_users: number }>(
    request,
    "/api/v1/public/users/stats",
    { method: "GET" },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "STATS_FAILED",
      "Failed to load stats.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
