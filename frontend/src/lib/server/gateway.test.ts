import { afterEach, describe, expect, it, vi } from "vitest";

import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

afterEach(() => {
  vi.unstubAllGlobals();
});

function req(headers: Record<string, string> = {}): Request {
  return new Request("https://app.test/api/bff/posts", { headers });
}

describe("proxyGateway when the gateway is unreachable", () => {
  it("returns a typed 503 instead of letting the fetch rejection escape", async () => {
    // Previously the rejected fetch propagated out of the route handler and
    // Next answered with a bare 500 and an EMPTY body, which the client's
    // envelope parser reported as "Unexpected response from server" —
    // indistinguishable from a real bug.
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("fetch failed")),
    );

    const { upstream, payload, setCookies } = await proxyGateway(req(), "/api/v1/posts");

    expect(upstream.status).toBe(503);
    expect(payload?.success).toBe(false);
    expect(payload?.error?.code).toBe("GATEWAY_UNAVAILABLE");
    expect(setCookies).toEqual([]);
  });

  it("produces a 503 envelope once a route maps it through toFailureResponse", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("ECONNREFUSED")));

    const { upstream, payload } = await proxyGateway(req(), "/api/v1/posts");
    const response = toFailureResponse(
      upstream.status,
      "LIST_FAILED",
      "Failed to list posts.",
      payload?.error,
    );

    expect(response.status).toBe(503);
    // The upstream code wins over the route's fallback, so callers can tell
    // "backend down" apart from "this request was bad".
    expect(JSON.parse(await response.text()).error.code).toBe("GATEWAY_UNAVAILABLE");
  });
});

describe("proxyGateway header handling", () => {
  it("forwards the browser cookie so the gateway sees the session", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: {} }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await proxyGateway(req({ cookie: "access_token=tok" }), "/api/v1/posts");

    const sentHeaders = fetchMock.mock.calls[0][1].headers as Headers;
    expect(sentHeaders.get("cookie")).toBe("access_token=tok");
    expect(sentHeaders.get("accept")).toBe("application/json");
  });

  it("tolerates a non-JSON upstream body by yielding a null payload", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("<html>502</html>", { status: 502 })),
    );

    const { upstream, payload } = await proxyGateway(req(), "/api/v1/posts");

    expect(upstream.status).toBe(502);
    expect(payload).toBeNull();
  });
});

describe("response helpers", () => {
  it("re-issues upstream cookies as HttpOnly + SameSite=Strict", () => {
    // The BFF→gateway hop is plain HTTP, so an upstream Set-Cookie may lack
    // Secure and is not pinned to Strict. We must not relay it verbatim.
    const response = toSuccessResponse({ ok: true }, 200, [
      "refresh_token=abc; Path=/; Max-Age=604800",
    ]);

    const cookie = response.cookies.get("refresh_token");
    expect(cookie?.value).toBe("abc");
    expect(cookie?.httpOnly).toBe(true);
    expect(cookie?.sameSite).toBe("strict");
    expect(cookie?.maxAge).toBe(604800);
  });

  it("preserves a deletion cookie so logout still clears the session", () => {
    const response = toSuccessResponse({ ok: true }, 200, [
      "refresh_token=; Path=/; Max-Age=0",
    ]);

    expect(response.cookies.get("refresh_token")?.maxAge).toBe(0);
  });

  it("prefers the upstream error code over the route fallback", async () => {
    const response = toFailureResponse(400, "FALLBACK", "Fallback message.", {
      code: "UPSTREAM_CODE",
      message: "Upstream message.",
    });

    expect(JSON.parse(await response.text()).error).toEqual({
      code: "UPSTREAM_CODE",
      message: "Upstream message.",
    });
  });

  it("uses the fallback when upstream supplied no error", async () => {
    const response = toFailureResponse(500, "FALLBACK", "Fallback message.");

    expect(JSON.parse(await response.text()).error).toEqual({
      code: "FALLBACK",
      message: "Fallback message.",
    });
  });
});
