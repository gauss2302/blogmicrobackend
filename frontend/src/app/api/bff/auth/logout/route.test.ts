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

function logoutRequest(cookie: string) {
  return new Request("https://app.test/api/bff/auth/logout", {
    method: "POST",
    headers: { cookie, "sec-fetch-site": "same-origin" },
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("POST /api/bff/auth/logout", () => {
  it("revokes by the refresh token when the access cookie has lapsed", async () => {
    const fetchMock = stubGateway(200, { success: true, data: null });

    const response = await POST(logoutRequest(`${AUTH_REFRESH_COOKIE_NAME}=r1`));

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledOnce();
    const init = fetchMock.mock.calls[0][1];
    expect(new Headers(init.headers).has("authorization")).toBe(false);
    expect(JSON.parse(String(init.body))).toEqual({ refresh_token: "r1" });
    expect(response.cookies.get(AUTH_REFRESH_COOKIE_NAME)?.maxAge).toBe(0);
  });

  it("clears its own cookies even when the gateway rate-limits the call", async () => {
    stubGateway(429, { success: false, error: { code: "RATE_LIMITED", message: "x" } });

    const response = await POST(logoutRequest(`${AUTH_REFRESH_COOKIE_NAME}=r1; access_token=a1`));

    expect(response.status).toBe(429);
    expect(response.cookies.get(AUTH_REFRESH_COOKIE_NAME)?.maxAge).toBe(0);
    expect(response.cookies.get("access_token")?.maxAge).toBe(0);
  });

  it("does not call the gateway without any session cookie", async () => {
    const fetchMock = stubGateway(200, { success: true, data: null });

    const response = await POST(logoutRequest("other=1"));

    expect(response.status).toBe(200);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
