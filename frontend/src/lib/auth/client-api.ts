"use client";

import {
  BFF_BASE_URL,
  OAUTH_CLIENT_STATE_STORAGE_KEY,
} from "@/lib/auth/client-constants";
import {
  exchangeAuthCodeSchema,
  type ExchangeAuthCodeInput,
  loginSchema,
  type LoginInput,
  registerSchema,
  type RegisterInput,
} from "@/lib/auth/schemas";
import type {
  APIEnvelope,
  APIFailure,
  GoogleAuthURLPayload,
  SessionPayload,
} from "@/lib/auth/types";
import { useAuthStore } from "@/lib/stores/auth-store";

let refreshPromise: Promise<SessionPayload | null> | null = null;

function getErrorMessage(payload: APIFailure | null, fallback: string) {
  return payload?.error?.message || fallback;
}

async function readAPIEnvelope<T>(response: Response): Promise<APIEnvelope<T>> {
  try {
    return (await response.json()) as APIEnvelope<T>;
  } catch {
    return {
      success: false,
      error: {
        code: "INTERNAL_ERROR",
        message: "Unexpected response from server.",
      },
    };
  }
}

async function postBFF<TResponse>(
  path: string,
  body: Record<string, unknown>,
  headers?: HeadersInit,
) {
  const response = await fetch(`${BFF_BASE_URL}${path}`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      ...headers,
    },
    credentials: "include",
    cache: "no-store",
    body: JSON.stringify(body),
  });

  const payload = await readAPIEnvelope<TResponse>(response);
  if (!payload.success) {
    throw new Error(getErrorMessage(payload, "Request failed."));
  }

  return payload.data;
}

export async function loginWithPassword(input: LoginInput) {
  const parsed = loginSchema.safeParse(input);
  if (!parsed.success) {
    throw new Error(parsed.error.issues[0]?.message || "Invalid login payload.");
  }

  const session = await postBFF<SessionPayload>("/auth/login", parsed.data);
  useAuthStore.getState().setSession(session);
  return session;
}

export async function registerWithPassword(input: RegisterInput) {
  const parsed = registerSchema.safeParse(input);
  if (!parsed.success) {
    throw new Error(
      parsed.error.issues[0]?.message || "Invalid registration payload.",
    );
  }

  const session = await postBFF<SessionPayload>("/auth/register", parsed.data);
  useAuthStore.getState().setSession(session);
  return session;
}

export async function getGoogleAuthURL(redirectURI: string, clientState: string) {
  const query = new URLSearchParams({
    platform: "web",
    redirect_uri: redirectURI,
    client_state: clientState,
  });

  const response = await fetch(`${BFF_BASE_URL}/auth/google?${query.toString()}`, {
    method: "GET",
    credentials: "include",
    cache: "no-store",
  });

  const payload = await readAPIEnvelope<GoogleAuthURLPayload>(response);
  if (!payload.success) {
    throw new Error(getErrorMessage(payload, "Failed to initialize Google OAuth."));
  }

  return payload.data;
}

export async function exchangeGoogleAuthCode(input: ExchangeAuthCodeInput) {
  const parsed = exchangeAuthCodeSchema.safeParse(input);
  if (!parsed.success) {
    throw new Error(parsed.error.issues[0]?.message || "Invalid auth code payload.");
  }

  const session = await postBFF<SessionPayload>("/auth/exchange", parsed.data);
  useAuthStore.getState().setSession(session);
  return session;
}

export async function refreshSession() {
  const { setRefreshing, setSession, clearSession } = useAuthStore.getState();
  setRefreshing(true);

  try {
    const session = await postBFF<SessionPayload>("/auth/refresh", {});
    setSession(session);
    return session;
  } catch {
    clearSession();
    return null;
  } finally {
    setRefreshing(false);
  }
}

export async function refreshSessionOnce() {
  if (!refreshPromise) {
    refreshPromise = refreshSession().finally(() => {
      refreshPromise = null;
    });
  }

  return refreshPromise;
}

export async function authenticatedFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const token = useAuthStore.getState().accessToken;
  const headers = new Headers(init.headers);

  if (token) {
    headers.set("authorization", `Bearer ${token}`);
  }

  const response = await fetch(path, {
    ...init,
    headers,
    credentials: "include",
    cache: "no-store",
  });

  if (response.status !== 401) {
    const payload = await readAPIEnvelope<T>(response);
    if (!payload.success) {
      throw new Error(getErrorMessage(payload, "Request failed."));
    }
    return payload.data;
  }

  const refreshed = await refreshSessionOnce();
  if (!refreshed) {
    throw new Error("Authentication expired.");
  }

  const retryHeaders = new Headers(init.headers);
  retryHeaders.set("authorization", `Bearer ${refreshed.accessToken}`);

  const retryResponse = await fetch(path, {
    ...init,
    headers: retryHeaders,
    credentials: "include",
    cache: "no-store",
  });
  const retryPayload = await readAPIEnvelope<T>(retryResponse);
  if (!retryPayload.success) {
    throw new Error(getErrorMessage(retryPayload, "Request failed after refresh."));
  }

  return retryPayload.data;
}

export async function logoutSession() {
  const token = useAuthStore.getState().accessToken;
  const headers: HeadersInit = {};
  if (token) {
    headers.authorization = `Bearer ${token}`;
  }

  let success = true;
  try {
    await postBFF<{ loggedOut: boolean }>("/auth/logout", {}, headers);
  } catch {
    success = false;
  } finally {
    useAuthStore.getState().clearSession();
    if (typeof window !== "undefined") {
      sessionStorage.removeItem(OAUTH_CLIENT_STATE_STORAGE_KEY);
    }
  }

  return success;
}
