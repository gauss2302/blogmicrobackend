import { describe, expect, it } from "vitest";

import { createPostSchema, updatePostSchema } from "@/lib/posts/schemas";

describe("createPostSchema", () => {
  it("accepts a minimal valid post", () => {
    const parsed = createPostSchema.parse({ title: "T", content: "C" });
    expect(parsed.published).toBe(true); // documented default
  });

  it("requires title and content", () => {
    expect(createPostSchema.safeParse({ title: "", content: "C" }).success).toBe(false);
    expect(createPostSchema.safeParse({ title: "T", content: "" }).success).toBe(false);
  });

  it("enforces the gateway's length bounds", () => {
    expect(createPostSchema.safeParse({ title: "x".repeat(201), content: "C" }).success).toBe(false);
    expect(createPostSchema.safeParse({ title: "T", content: "x".repeat(50_001) }).success).toBe(false);
  });

  it("rejects a slug shorter than the backend minimum", () => {
    expect(createPostSchema.safeParse({ title: "T", content: "C", slug: "ab" }).success).toBe(false);
  });
});

describe("updatePostSchema", () => {
  it("allows toggling a single field", () => {
    expect(updatePostSchema.safeParse({ published: false }).success).toBe(true);
  });

  it("rejects an empty patch", () => {
    expect(updatePostSchema.safeParse({}).success).toBe(false);
  });

  it("has no default for published, so an omitted key stays omitted", () => {
    // A default here would silently republish a draft on any unrelated edit.
    const parsed = updatePostSchema.parse({ title: "New title" });
    expect(parsed.published).toBeUndefined();
    expect(Object.keys(parsed)).toEqual(["title"]);
  });

  it("still applies create-level bounds to the fields it does receive", () => {
    expect(updatePostSchema.safeParse({ title: "" }).success).toBe(false);
    expect(updatePostSchema.safeParse({ title: "x".repeat(201) }).success).toBe(false);
  });
});
