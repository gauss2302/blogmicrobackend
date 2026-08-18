import { rejectCrossSite } from "@/lib/server/csrf";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import { authorizationHeader, clearSessionCookies } from "@/lib/server/session";
import { updateUserSchema } from "@/lib/users/schemas";
import type { User } from "@/lib/users/types";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  if (!id) {
    return toFailureResponse(400, "INVALID_REQUEST", "User ID is required.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { upstream, payload } = await proxyGateway<User>(
    request,
    `/api/v1/users/${encodeURIComponent(id)}`,
    {
      method: "GET",
      headers: { authorization },
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "USER_NOT_FOUND",
      "User not found.",
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
    return toFailureResponse(400, "INVALID_REQUEST", "User ID is required.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const rawBody = await request.json().catch(() => null);
  const parsed = updateUserSchema.safeParse(rawBody);
  if (!parsed.success) {
    return toFailureResponse(
      400,
      "INVALID_REQUEST",
      parsed.error.issues[0]?.message || "Invalid profile payload.",
    );
  }

  // Only forward keys the caller actually set — an absent key means "unchanged"
  // upstream. Note we never forward an id or actor_id from the body: the
  // gateway derives both from the path and the access token, and user-service
  // enforces that they match.
  const body = Object.fromEntries(
    Object.entries(parsed.data).filter(([, value]) => value !== undefined),
  );

  const { upstream, payload } = await proxyGateway<User>(
    request,
    `/api/v1/users/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      headers: { authorization },
      body: JSON.stringify(body),
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "PROFILE_UPDATE_FAILED",
      "Failed to update profile.",
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
    return toFailureResponse(400, "INVALID_REQUEST", "User ID is required.");
  }

  const authorization = authorizationHeader(request);
  if (!authorization) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { upstream, payload } = await proxyGateway<unknown>(
    request,
    `/api/v1/users/${encodeURIComponent(id)}`,
    {
      method: "DELETE",
      headers: { authorization },
    },
  );

  if (!payload?.success) {
    return toFailureResponse(
      upstream.status,
      "ACCOUNT_DELETE_FAILED",
      "Failed to delete account.",
      payload?.error,
    );
  }

  // The account is gone, so the session cookies now reference a user that no
  // longer exists. Drop them here rather than relying on the client to call
  // logout afterwards — a failed follow-up would leave the browser holding
  // credentials for a deleted account.
  const response = toSuccessResponse(null, upstream.status);
  clearSessionCookies(response);
  return response;
}
