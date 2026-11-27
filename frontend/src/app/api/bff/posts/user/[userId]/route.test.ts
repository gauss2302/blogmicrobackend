import { afterEach, describe, expect, it, vi } from "vitest";

import { AUTH_ACCESS_COOKIE_NAME } from "@/lib/server/session";

import { GET } from "./route";

afterEach(() => {
  vi.unstubAllGlobals();
});

async function sentAuthorization(cookie?: string): Promise<string | null> {
  const fetchMock = vi.fn().mockResolvedValue(
    new Response(JSON.stringify({ success: true, data: { posts: [], limit: 20, offset: 0, total: 0 } }), {
      status: 200,
    }),
  );
  vi.stubGlobal("fetch", fetchMock);

  const request = new Request("https://app.test/api/bff/posts/user/u1", {
    headers: cookie ? { cookie } : {},
  });
  const response = await GET(request, { params: Promise.resolve({ userId: "u1" }) });

  expect(response.status).toBe(200);
  expect(fetchMock.mock.calls[0][0]).toContain("/api/v1/public/posts/user/u1");
  return (fetchMock.mock.calls[0][1].headers as Headers).get("authorization");
}

describe("GET /api/bff/posts/user/[userId]", () => {
  it("sends the caller's token so the author's drafts come back", async () => {
    expect(await sentAuthorization(`${AUTH_ACCESS_COOKIE_NAME}=tok`)).toBe("Bearer tok");
  });

  it("stays anonymous without a session", async () => {
    expect(await sentAuthorization()).toBeNull();
  });
});
