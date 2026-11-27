import { describe, expect, it } from "vitest";

import { safeNextPath } from "@/lib/auth/next-path";

const origin = "https://app.example";
// The form reads ?next= through URLSearchParams, which decodes it first.
const next = (query: string) => safeNextPath(new URLSearchParams(query).get("next"), origin);

describe("safeNextPath", () => {
  it.each([
    "next=/%5Cevil.example",
    "next=/%09/evil.example",
    "next=//evil.example",
    "next=https://evil.example",
    "next=javascript:alert(1)",
  ])("rejects %s", (query) => {
    expect(next(query)).toBe("/app");
  });

  it("keeps a same-origin path with its query", () => {
    expect(next("next=%2Fapp%2Fprofile%3Fx%3D1")).toBe("/app/profile?x=1");
  });

  it("falls back to /app without next", () => {
    expect(next("")).toBe("/app");
  });
});
