import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader } from "@/lib/server/session";
import type { UserProfile } from "@/lib/users/types";

/**
 * Public profile view.
 *
 * The upstream route sits behind `OptionalAuthMiddleware`, so it works with or
 * without a session — an anonymous visitor gets the profile, a signed-in one
 * may additionally get viewer-relative fields (e.g. follow state). We therefore
 * forward the Authorization header when a session exists but never require it.
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

  const { upstream, payload } = await proxyGateway<UserProfile>(
    request,
    `/api/v1/public/users/${encodeURIComponent(id)}/profile`,
    {
      method: "GET",
      ...(authorization ? { headers: { authorization } } : {}),
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "PROFILE_NOT_FOUND",
      "Profile not found.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
