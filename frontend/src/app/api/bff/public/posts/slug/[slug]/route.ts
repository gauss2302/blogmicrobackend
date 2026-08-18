import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader } from "@/lib/server/session";

/**
 * Public read behind the gateway's OptionalAuthMiddleware: works signed out,
 * and a session (when present) may enrich the result. The Authorization header
 * is therefore forwarded opportunistically and never required.
 */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ slug: string }> },
) {
  const { slug } = await params;
  if (!slug) {
    return toFailureResponse(400, "INVALID_REQUEST", "Slug is required.");
  }

  const authorization = authorizationHeader(request);

  const { upstream, payload } = await proxyGateway<unknown>(
    request,
    `/api/v1/public/posts/slug/${encodeURIComponent(slug)}`,
    {
      method: "GET",
      ...(authorization ? { headers: { authorization } } : {}),
    },
  );

  if (!payload?.success || payload.data === undefined) {
    return toFailureResponse(
      upstream.status,
      "POST_NOT_FOUND",
      "Post not found.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
