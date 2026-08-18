import { updatePostSchema } from "@/lib/posts/schemas";
import { rejectCrossSite } from "@/lib/server/csrf";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader } from "@/lib/server/session";

interface PostData {
  id: string;
  user_id: string;
  title: string;
  content: string;
  slug: string;
  published: boolean;
  created_at: string;
  updated_at: string;
}

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  if (!id) {
    return toFailureResponse(400, "INVALID_REQUEST", "Post ID is required.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { upstream, payload } = await proxyGateway<PostData>(request, `/api/v1/posts/${encodeURIComponent(id)}`, {
    method: "GET",
    headers: { authorization },
  });

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "POST_NOT_FOUND",
      "Post not found.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}

export async function PUT(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const blocked = rejectCrossSite(request);
  if (blocked) return blocked;

  const { id } = await params;
  if (!id) {
    return toFailureResponse(400, "INVALID_REQUEST", "Post ID is required.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const rawBody = await request.json().catch(() => null);
  const parsed = updatePostSchema.safeParse(rawBody);
  if (!parsed.success) {
    return toFailureResponse(
      400,
      "INVALID_REQUEST",
      parsed.error.issues[0]?.message || "Invalid post payload.",
    );
  }

  // Strip undefined keys so an omitted field stays omitted on the wire. The
  // gateway distinguishes "absent" from "present", and JSON.stringify would
  // drop undefined anyway — being explicit keeps that contract obvious.
  const body = Object.fromEntries(
    Object.entries(parsed.data).filter(([, value]) => value !== undefined),
  );

  const { upstream, payload } = await proxyGateway<PostData>(
    request,
    `/api/v1/posts/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      headers: { authorization },
      body: JSON.stringify(body),
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "POST_UPDATE_FAILED",
      "Failed to update post.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}

export async function DELETE(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const blocked = rejectCrossSite(request);
  if (blocked) return blocked;

  const { id } = await params;
  if (!id) {
    return toFailureResponse(400, "INVALID_REQUEST", "Post ID is required.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { upstream, payload } = await proxyGateway<unknown>(
    request,
    `/api/v1/posts/${encodeURIComponent(id)}`,
    {
      method: "DELETE",
      headers: { authorization },
    },
  );

  if (!payload?.success) {
    return toFailureResponse(
      upstream.status,
      "POST_DELETE_FAILED",
      "Failed to delete post.",
      payload?.error,
    );
  }

  return toSuccessResponse(null, upstream.status);
}
