import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader } from "@/lib/server/session";

/**
 * Public user search. The gateway clamps limit to 1..100 and offset to a
 * maximum; we mirror the clamp so the UI cannot silently request more than it
 * will receive.
 */
export async function GET(request: Request) {
  const url = new URL(request.url);
  const q = (url.searchParams.get("q") ?? "").trim();
  if (!q) {
    return toFailureResponse(400, "INVALID_REQUEST", "Query is required.");
  }

  const rawLimit = Number(url.searchParams.get("limit") ?? 20);
  const limit = Number.isFinite(rawLimit)
    ? Math.min(100, Math.max(1, Math.trunc(rawLimit)))
    : 20;
  const rawOffset = Number(url.searchParams.get("offset") ?? 0);
  const offset = Number.isFinite(rawOffset) ? Math.max(0, Math.trunc(rawOffset)) : 0;

  const query = new URLSearchParams({
    q,
    limit: String(limit),
    offset: String(offset),
  });

  const authorization = authorizationHeader(request);

  const { upstream, payload } = await proxyGateway<unknown>(
    request,
    `/api/v1/public/users/search?${query}`,
    {
      method: "GET",
      ...(authorization ? { headers: { authorization } } : {}),
    },
  );

  if (!payload?.success || payload.data === undefined) {
    return toFailureResponse(
      upstream.status,
      "USER_SEARCH_FAILED",
      "Search failed.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
