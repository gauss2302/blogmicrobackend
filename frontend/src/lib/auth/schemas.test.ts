import { describe, expect, it } from "vitest";

import { loginSchema, registerSchema } from "@/lib/auth/schemas";

const base = { email: "u@example.test", name: "U" };

describe.each([
  ["loginSchema", loginSchema],
  ["registerSchema", registerSchema],
])("%s password", (_, schema) => {
  it("accepts 72 bytes and rejects 73", () => {
    expect(schema.safeParse({ ...base, password: "p".repeat(72) }).success).toBe(true);
    const result = schema.safeParse({ ...base, password: "p".repeat(73) });
    expect(result.success).toBe(false);
    expect(result.error?.issues[0]?.message).toBe("Password must be at most 72 bytes.");
  });

  it("counts Cyrillic as 2 bytes per letter", () => {
    expect(schema.safeParse({ ...base, password: "п".repeat(36) }).success).toBe(true);
    expect(schema.safeParse({ ...base, password: "п".repeat(37) }).success).toBe(false);
  });
});

describe("registerSchema name", () => {
  it("is limited to 100 UTF-8 bytes", () => {
    const valid = { email: "u@example.test", password: "password1" };
    expect(registerSchema.safeParse({ ...valid, name: "я".repeat(50) }).success).toBe(true);
    expect(registerSchema.safeParse({ ...valid, name: "я".repeat(51) }).success).toBe(false);
  });
});
