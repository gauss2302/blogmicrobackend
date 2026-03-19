import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const authorization = request.headers.get("authorization");
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { id } = await params;
  if (!id) {
    return toFailureResponse(400, "INVALID_REQUEST", "User ID is required.");
  }

  const { upstream, payload } = await proxyGateway<unknown>(request, `/api/v1/users/${encodeURIComponent(id)}/follow`, {
    method: "POST",
    headers: { authorization },
  });

  if (!payload?.success) {
    return toFailureResponse(
      upstream.status,
      "FOLLOW_FAILED",
      "Failed to follow user.",
      payload?.error,
    );
  }

  return toSuccessResponse(null, upstream.status);
}

export async function DELETE(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const authorization = request.headers.get("authorization");
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { id } = await params;
  if (!id) {
    return toFailureResponse(400, "INVALID_REQUEST", "User ID is required.");
  }

  const { upstream, payload } = await proxyGateway<unknown>(request, `/api/v1/users/${encodeURIComponent(id)}/follow`, {
    method: "DELETE",
    headers: { authorization },
  });

  if (!payload?.success) {
    return toFailureResponse(
      upstream.status,
      "UNFOLLOW_FAILED",
      "Failed to unfollow user.",
      payload?.error,
    );
  }

  return toSuccessResponse(null, upstream.status);
}
