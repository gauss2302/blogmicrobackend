import { describe, expect, it } from "vitest";

import { buildCsp } from "@/proxy";

describe("buildCsp", () => {
  it("keeps the production policy exactly as before, without unsafe-eval", () => {
    expect(buildCsp("abc", false)).toBe(
      "default-src 'self'; script-src 'self' 'nonce-abc'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com data:; img-src 'self' data: https:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'",
    );
  });

  it("adds unsafe-eval to script-src in development only", () => {
    expect(buildCsp("abc", true)).toContain("script-src 'self' 'nonce-abc' 'unsafe-eval';");
  });

  it("defaults to the current NODE_ENV, which is not development under vitest", () => {
    expect(buildCsp("abc")).not.toContain("unsafe-eval");
  });
});
