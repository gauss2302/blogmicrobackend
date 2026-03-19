"use client";

import { BFF_BASE_URL } from "@/lib/auth/client-constants";
import { authenticatedFetch } from "@/lib/auth/client-api";

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

export interface SearchResponse {
  users: SearchUserHit[];
  posts: SearchPostHit[];
  users_next_cursor: string;
  posts_next_cursor: string;
  users_partial: boolean;
  posts_partial: boolean;
}

export interface SearchParams {
  q: string;
  usersLimit?: number;
  postsLimit?: number;
  usersCursor?: string;
  postsCursor?: string;
}

export async function search(params: SearchParams): Promise<SearchResponse> {
  const { q, usersLimit = 20, postsLimit = 20, usersCursor = "", postsCursor = "" } = params;
  const sp = new URLSearchParams({ q: q.trim(), users_limit: String(usersLimit), posts_limit: String(postsLimit) });
  if (usersCursor) sp.set("users_cursor", usersCursor);
  if (postsCursor) sp.set("posts_cursor", postsCursor);
  return authenticatedFetch<SearchResponse>(`${BFF_BASE_URL}/search?${sp.toString()}`, { method: "GET" });
}

export async function followUser(userId: string): Promise<void> {
  await authenticatedFetch<unknown>(`${BFF_BASE_URL}/users/${encodeURIComponent(userId)}/follow`, {
    method: "POST",
  });
}

export async function unfollowUser(userId: string): Promise<void> {
  await authenticatedFetch<unknown>(`${BFF_BASE_URL}/users/${encodeURIComponent(userId)}/follow`, {
    method: "DELETE",
  });
}
