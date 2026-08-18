import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader } from "@/lib/server/session";
import type { ListFollowResponse } from "@/lib/users/types";

/**
 * Cursor-paginated followers list.
 *
 * The gateway caps `limit` at 100 and falls back to 20 for anything out of
 * range; we clamp here too so the UI cannot silently ask for more than it gets.
 */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  if (!id) {
    return toFailureResponse(400, "INVALID_REQUEST", "User ID is required.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const url = new URL(request.url);
  const requested = Number(url.searchParams.get("limit") ?? 20);
  const limit = Number.isFinite(requested)
    ? Math.min(100, Math.max(1, Math.trunc(requested)))
    : 20;
  const cursor = url.searchParams.get("cursor") ?? "";

  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set("cursor", cursor);

  const { upstream, payload } = await proxyGateway<ListFollowResponse>(
    request,
    `/api/v1/users/${encodeURIComponent(id)}/followers?${query}`,
    {
      method: "GET",
      headers: { authorization },
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "FOLLOWERS_FAILED",
      "Failed to load followers.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
