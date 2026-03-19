"use client";

import { BFF_BASE_URL } from "@/lib/auth/client-constants";
import { authenticatedFetch } from "@/lib/auth/client-api";
import { createPostSchema, type CreatePostInput } from "@/lib/posts/schemas";
import type { ListPostsResponse, Post } from "@/lib/posts/types";

interface APIEnvelope<T> {
  success: boolean;
  data?: T;
  error?: { code?: string; message?: string };
}

async function getBFF<T>(path: string): Promise<T> {
  const response = await fetch(`${BFF_BASE_URL}${path}`, {
    method: "GET",
    credentials: "include",
    cache: "no-store",
  });
  const payload = (await response.json()) as APIEnvelope<T>;
  if (!payload.success || payload.data === undefined) {
    throw new Error(payload.error?.message ?? "Request failed.");
  }
  return payload.data;
}

export interface ListPostsParams {
  limit?: number;
  offset?: number;
  publishedOnly?: boolean;
}

export async function listPosts(params: ListPostsParams = {}): Promise<ListPostsResponse> {
  const { limit = 20, offset = 0, publishedOnly = true } = params;
  const q = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
    published_only: String(publishedOnly),
  });
  return getBFF<ListPostsResponse>(`/posts?${q.toString()}`);
}

export async function listUserPosts(
  userId: string,
  params: ListPostsParams = {},
): Promise<ListPostsResponse> {
  const { limit = 20, offset = 0 } = params;
  const q = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  return getBFF<ListPostsResponse>(`/posts/user/${encodeURIComponent(userId)}?${q.toString()}`);
}

export async function getPost(id: string): Promise<Post> {
  if (!id.trim()) {
    throw new Error("Post ID is required.");
  }
  return authenticatedFetch<Post>(`${BFF_BASE_URL}/posts/${encodeURIComponent(id)}`, {
    method: "GET",
  });
}

export async function createPost(input: CreatePostInput): Promise<Post> {
  const parsed = createPostSchema.safeParse(input);
  if (!parsed.success) {
    throw new Error(parsed.error.issues[0]?.message ?? "Invalid post data.");
  }
  const body: Record<string, unknown> = {
    title: parsed.data.title,
    content: parsed.data.content,
    published: parsed.data.published ?? true,
  };
  if (parsed.data.slug && parsed.data.slug.trim()) {
    body.slug = parsed.data.slug.trim();
  }
  return authenticatedFetch<Post>(`${BFF_BASE_URL}/posts`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}
