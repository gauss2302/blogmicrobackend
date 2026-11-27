import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { refreshSession } from "@/lib/auth/client-api";
import { useAuthStore } from "@/lib/stores/auth-store";

const session = {
  tokenType: "Bearer",
  expiresIn: 900,
  user: { id: "u1", email: "u@example.test" },
};

function failWith(status: number) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      Response.json({ success: false, error: { code: "REFRESH_FAILED", message: "Token refresh failed." } }, { status }),
    ),
  );
}

describe("refreshSession", () => {
  beforeEach(() => {
    useAuthStore.getState().setSession(session);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("ends the session when the refresh token is rejected (401)", async () => {
    failWith(401);
    await expect(refreshSession()).resolves.toBeNull();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });

  it("ends the session when there is no refresh token (400)", async () => {
    failWith(400);
    await expect(refreshSession()).resolves.toBeNull();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });

  it("keeps the session when the backend is unavailable (503)", async () => {
    failWith(503);
    await expect(refreshSession()).rejects.toThrow("Token refresh failed.");
    expect(useAuthStore.getState().isAuthenticated).toBe(true);
    expect(useAuthStore.getState().isRefreshing).toBe(false);
  });

  it("keeps the session on a network error", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("Failed to fetch"))));
    await expect(refreshSession()).rejects.toThrow("Failed to fetch");
    expect(useAuthStore.getState().isAuthenticated).toBe(true);
  });

  it("stores the new session on success", async () => {
    useAuthStore.getState().clearSession();
    vi.stubGlobal("fetch", vi.fn(async () => Response.json({ success: true, data: session })));
    await expect(refreshSession()).resolves.toEqual(session);
    expect(useAuthStore.getState().user?.id).toBe("u1");
  });
});
