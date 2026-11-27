import { afterEach, describe, expect, it, vi } from "vitest";

import { AUTH_REFRESH_COOKIE_NAME } from "@/lib/auth/server-constants";

import { POST } from "./route";

function stubGateway(status: number, body: unknown) {
  const fetchMock = vi.fn<(url: string, init: RequestInit) => Promise<Response>>(async () =>
    new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function refreshRequest(cookie: string) {
  return new Request("https://app.test/api/bff/auth/refresh", {
    method: "POST",
    headers: { cookie, "sec-fetch-site": "same-origin" },
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("POST /api/bff/auth/refresh", () => {
  it("sends the refresh cookie in the body, for a gateway without cookie mode", async () => {
    const fetchMock = stubGateway(200, {
      success: true,
      data: { access_token: "a2", refresh_token: "r2", token_type: "Bearer", expires_in: 900, user: { id: "u1" } },
    });

    const response = await POST(refreshRequest(`${AUTH_REFRESH_COOKIE_NAME}=r1`));

    expect(response.status).toBe(200);
    expect(JSON.parse(String(fetchMock.mock.calls[0][1].body))).toEqual({ refresh_token: "r1" });
    expect(response.cookies.get(AUTH_REFRESH_COOKIE_NAME)?.value).toBe("r2");
  });
});
