import path from "node:path";

import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // These suites cover pure server-side logic (cookies, CSRF, schemas), so
    // the default node environment is correct — no DOM needed.
    environment: "node",
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      include: ["src/lib/**"],
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
});
