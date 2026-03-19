import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export interface SearchUserHit {
  id: string;
  name: string;
  picture: string;
  bio: string;
}

export interface SearchPostHit {
  id: string;
  user_id: string;
  title: string;
  slug: string;
  content_preview: string;
  published: boolean;
}

export interface SearchData {
  users: SearchUserHit[];
  posts: SearchPostHit[];
  users_next_cursor: string;
  posts_next_cursor: string;
  users_partial: boolean;
  posts_partial: boolean;
}

export async function GET(request: Request) {
  const authorization = request.headers.get("authorization");
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { searchParams } = new URL(request.url);
  const q = searchParams.get("q");
  if (!q || q.trim() === "") {
    return toFailureResponse(400, "MISSING_QUERY", "Search query is required.");
  }

  const usersLimit = searchParams.get("users_limit") ?? "20";
  const postsLimit = searchParams.get("posts_limit") ?? "20";
  const usersCursor = searchParams.get("users_cursor") ?? "";
  const postsCursor = searchParams.get("posts_cursor") ?? "";

  const path = `/api/v1/search?q=${encodeURIComponent(q.trim())}&users_limit=${usersLimit}&posts_limit=${postsLimit}${usersCursor ? `&users_cursor=${encodeURIComponent(usersCursor)}` : ""}${postsCursor ? `&posts_cursor=${encodeURIComponent(postsCursor)}` : ""}`;

  const { upstream, payload } = await proxyGateway<SearchData>(request, path, {
    method: "GET",
    headers: { authorization },
  });

  if (!payload?.success || payload.data === undefined) {
    return toFailureResponse(
      upstream.status,
      "SEARCH_FAILED",
      "Search failed.",
      payload?.error,
    );
  }

  return toSuccessResponse(payload.data, upstream.status);
}
