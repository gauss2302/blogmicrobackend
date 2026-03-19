import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

/** Backend list response shape (aligned with api-gateway ListPostsResponse). */
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

/** Backend single post response (aligned with api-gateway PostResponse). */
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

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const limit = searchParams.get("limit") ?? "20";
  const offset = searchParams.get("offset") ?? "0";
  const publishedOnly = searchParams.get("published_only") ?? "true";
  const path = `/api/v1/public/posts?limit=${limit}&offset=${offset}&published_only=${publishedOnly}`;

  const { upstream, payload } = await proxyGateway<ListPostsData>(request, path, {
    method: "GET",
  });

  if (!payload?.success || payload.data === undefined) {
    return toFailureResponse(
      upstream.status,
      "LIST_FAILED",
      "Failed to retrieve posts.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}

export async function POST(request: Request) {
  const authorization = request.headers.get("authorization");
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const rawBody = await request.json().catch(() => null);
  if (!rawBody || typeof rawBody !== "object") {
    return toFailureResponse(400, "INVALID_REQUEST", "Invalid request body.");
  }

  const { upstream, payload } = await proxyGateway<PostData>(request, "/api/v1/posts", {
    method: "POST",
    headers: { authorization },
    body: JSON.stringify(rawBody),
  });

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "CREATE_FAILED",
      "Failed to create post.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
