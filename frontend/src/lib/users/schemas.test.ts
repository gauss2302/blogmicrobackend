import { describe, expect, it } from "vitest";

import { updateUserSchema } from "@/lib/users/schemas";

describe("updateUserSchema", () => {
  it("accepts a single field — omitted keys mean 'leave unchanged'", () => {
    expect(updateUserSchema.safeParse({ name: "New Name" }).success).toBe(true);
  });

  it("rejects an entirely empty patch, which would be a pointless round-trip", () => {
    expect(updateUserSchema.safeParse({}).success).toBe(false);
  });

  it("rejects an empty name rather than silently blanking the profile", () => {
    expect(updateUserSchema.safeParse({ name: "" }).success).toBe(false);
  });

  it("allows clearing optional URL fields with an empty string", () => {
    // "" is how the user removes a website; it is distinct from omitting the key.
    expect(updateUserSchema.safeParse({ website: "" }).success).toBe(true);
    expect(updateUserSchema.safeParse({ picture: "" }).success).toBe(true);
  });

  it("rejects a malformed website URL", () => {
    expect(updateUserSchema.safeParse({ website: "not a url" }).success).toBe(false);
  });

  it("accepts a well-formed website URL", () => {
    expect(updateUserSchema.safeParse({ website: "https://example.com" }).success).toBe(true);
  });

  it("enforces the bio length ceiling", () => {
    expect(updateUserSchema.safeParse({ bio: "x".repeat(500) }).success).toBe(true);
    expect(updateUserSchema.safeParse({ bio: "x".repeat(501) }).success).toBe(false);
  });

  it("strips surrounding whitespace", () => {
    const parsed = updateUserSchema.parse({ name: "  Spaced  " });
    expect(parsed.name).toBe("Spaced");
  });

  it("ignores id and actor_id if a caller tries to smuggle them in", () => {
    // These must come from the URL path and access token. Accepting either
    // from the body would be an authorization hole.
    const parsed = updateUserSchema.parse({
      name: "Ok",
      id: "someone-else",
      actor_id: "someone-else",
    } as Record<string, unknown>);

    expect(parsed).not.toHaveProperty("id");
    expect(parsed).not.toHaveProperty("actor_id");
  });

  it("counts name, bio and location in UTF-8 bytes like user-service", () => {
    // Cyrillic letters are 2 bytes each.
    expect(updateUserSchema.safeParse({ name: "я".repeat(50) }).success).toBe(true);
    expect(updateUserSchema.safeParse({ name: "я".repeat(51) }).success).toBe(false);
    expect(updateUserSchema.safeParse({ bio: "я".repeat(250) }).success).toBe(true);
    expect(updateUserSchema.safeParse({ bio: "я".repeat(251) }).success).toBe(false);
    expect(updateUserSchema.safeParse({ location: "я".repeat(50) }).success).toBe(true);
    expect(updateUserSchema.safeParse({ location: "я".repeat(51) }).success).toBe(false);
  });

  it("accepts only http and https websites", () => {
    expect(updateUserSchema.safeParse({ website: "http://example.com" }).success).toBe(true);
    expect(updateUserSchema.safeParse({ website: "ftp://example.com" }).success).toBe(false);
    expect(updateUserSchema.safeParse({ website: "javascript:alert(1)" }).success).toBe(false);
  });
});
