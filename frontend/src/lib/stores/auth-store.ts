"use client";

import { create } from "zustand";

import type { SessionPayload, UserInfo } from "@/lib/auth/types";

type AuthState = {
  accessToken: string | null;
  tokenType: string | null;
  expiresIn: number | null;
  user: UserInfo | null;
  isRefreshing: boolean;
  setSession: (session: SessionPayload) => void;
  clearSession: () => void;
  setRefreshing: (isRefreshing: boolean) => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  tokenType: null,
  expiresIn: null,
  user: null,
  isRefreshing: false,
  setSession: (session) =>
    set({
      accessToken: session.accessToken,
      tokenType: session.tokenType,
      expiresIn: session.expiresIn,
      user: session.user,
      isRefreshing: false,
    }),
  clearSession: () =>
    set({
      accessToken: null,
      tokenType: null,
      expiresIn: null,
      user: null,
      isRefreshing: false,
    }),
  setRefreshing: (isRefreshing) => set({ isRefreshing }),
}));
