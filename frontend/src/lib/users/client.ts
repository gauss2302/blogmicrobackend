"use client";

import { authenticatedFetch } from "@/lib/auth/client-api";
import { BFF_BASE_URL } from "@/lib/auth/client-constants";
import { updateUserSchema, type UpdateUserInput } from "@/lib/users/schemas";
import type { ListFollowResponse, User, UserProfile } from "@/lib/users/types";

interface APIEnvelope<T> {
  success: boolean;
  data?: T;
  error?: { code?: string; message?: string };
}

async function getPublicBFF<T>(path: string): Promise<T> {
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

/** Authenticated read of a user record (includes email). */
export async function getUser(id: string): Promise<User> {
  return authenticatedFetch<User>(`${BFF_BASE_URL}/users/${encodeURIComponent(id)}`, {
    method: "GET",
  });
}

/**
 * Updates the signed-in user's profile.
 *
 * Validated client-side before the request so the user gets an immediate,
 * field-level message; the BFF re-validates with the same schema because
 * client-side checks are advisory only.
 */
export async function updateUser(id: string, input: UpdateUserInput): Promise<User> {
  const result = updateUserSchema.safeParse(input);
  if (!result.success) {
    // ZodError.message is the whole issue list as JSON; show the first issue.
    throw new Error(result.error.issues[0]?.message ?? "Invalid profile.");
  }
  const parsed = result.data;
  return authenticatedFetch<User>(`${BFF_BASE_URL}/users/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(parsed),
  });
}

export interface FollowListParams {
  limit?: number;
  cursor?: string;
}

function followListQuery({ limit = 20, cursor }: FollowListParams): string {
  const q = new URLSearchParams({ limit: String(limit) });
  if (cursor) q.set("cursor", cursor);
  return q.toString();
}

export async function listFollowers(
  userId: string,
  params: FollowListParams = {},
): Promise<ListFollowResponse> {
  return authenticatedFetch<ListFollowResponse>(
    `${BFF_BASE_URL}/users/${encodeURIComponent(userId)}/followers?${followListQuery(params)}`,
    { method: "GET" },
  );
}

export async function listFollowing(
  userId: string,
  params: FollowListParams = {},
): Promise<ListFollowResponse> {
  return authenticatedFetch<ListFollowResponse>(
    `${BFF_BASE_URL}/users/${encodeURIComponent(userId)}/following?${followListQuery(params)}`,
    { method: "GET" },
  );
}

/** Public profile — works signed out, so it does not go through authenticatedFetch. */
export async function getUserProfile(id: string): Promise<UserProfile> {
  return getPublicBFF<UserProfile>(`/public/users/${encodeURIComponent(id)}/profile`);
}

/**
 * Permanently deletes the signed-in user's account.
 *
 * The BFF clears the session cookies as part of the response, so the caller
 * only needs to reset local state and navigate away.
 */
export async function deleteAccount(id: string): Promise<void> {
  await authenticatedFetch<null>(`${BFF_BASE_URL}/users/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}
