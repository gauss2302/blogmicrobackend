"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";
import { THEME_STORAGE_KEY, type ThemePreference } from "@/lib/theme/constants";

type ThemeState = {
  preference: ThemePreference;
  setPreference: (preference: ThemePreference) => void;
};

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      preference: "dark",
      setPreference: (preference) => set({ preference }),
    }),
    { name: THEME_STORAGE_KEY },
  ),
);
