import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

interface ListPostsData {
  posts: Array<{
    id: string;
    user_id: string;
    title: string;
    slug: string;
    published: boolean;
    created_at: string;
    updated_at: string;
  }>;
  limit: number;
  offset: number;
  total: number;
}

export async function GET(
  request: Request,
  { params }: { params: Promise<{ userId: string }> },
) {
  const { userId } = await params;
  if (!userId) {
    return toFailureResponse(400, "INVALID_REQUEST", "User ID is required.");
  }

  const { searchParams } = new URL(request.url);
  const limit = searchParams.get("limit") ?? "20";
  const offset = searchParams.get("offset") ?? "0";
  const path = `/api/v1/public/posts/user/${encodeURIComponent(userId)}?limit=${limit}&offset=${offset}`;

  const { upstream, payload } = await proxyGateway<ListPostsData>(request, path, {
    method: "GET",
  });

  if (!payload?.success || payload.data === undefined) {
    return toFailureResponse(
      upstream.status,
      "USER_POSTS_FAILED",
      "Failed to retrieve user posts.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
