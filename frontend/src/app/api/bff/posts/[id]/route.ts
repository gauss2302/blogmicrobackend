import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

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

  const authorization = request.headers.get("authorization");
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
