"use client";

import { create } from "zustand";

import type { SessionPayload, UserInfo } from "@/lib/auth/types";

/**
 * Client-visible session state.
 *
 * Holds no credential: the access token lives in an HttpOnly cookie managed by
 * the BFF, so an XSS payload reading this store learns nothing it can replay.
 * `isAuthenticated` is derived from having a user, not from holding a token.
 */
type AuthState = {
  isAuthenticated: boolean;
  tokenType: string | null;
  expiresIn: number | null;
  user: UserInfo | null;
  isRefreshing: boolean;
  setSession: (session: SessionPayload) => void;
  clearSession: () => void;
  setRefreshing: (isRefreshing: boolean) => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  tokenType: null,
  expiresIn: null,
  user: null,
  isRefreshing: false,
  setSession: (session) =>
    set({
      isAuthenticated: true,
      tokenType: session.tokenType,
      expiresIn: session.expiresIn,
      user: session.user,
      isRefreshing: false,
    }),
  clearSession: () =>
    set({
      isAuthenticated: false,
      tokenType: null,
      expiresIn: null,
      user: null,
      isRefreshing: false,
    }),
  setRefreshing: (isRefreshing) => set({ isRefreshing }),
}));
