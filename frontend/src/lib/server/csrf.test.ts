import { describe, expect, it } from "vitest";

import { isCrossSiteRequest, rejectCrossSite } from "@/lib/server/csrf";

function req(headers: Record<string, string>): Request {
  return new Request("https://app.test/api/bff/posts", { method: "POST", headers });
}

describe("isCrossSiteRequest", () => {
  it("allows same-origin requests from our own SPA", () => {
    expect(isCrossSiteRequest(req({ "sec-fetch-site": "same-origin" }))).toBe(false);
  });

  it("allows 'none' — a direct user action such as an address-bar hit", () => {
    expect(isCrossSiteRequest(req({ "sec-fetch-site": "none" }))).toBe(false);
  });

  it("rejects cross-site", () => {
    expect(isCrossSiteRequest(req({ "sec-fetch-site": "cross-site" }))).toBe(true);
  });

  it("rejects same-site, which still covers a different subdomain", () => {
    // SameSite=Strict cookies WOULD be sent here, so this is exactly the case
    // the cookie attribute alone does not protect against.
    expect(isCrossSiteRequest(req({ "sec-fetch-site": "same-site" }))).toBe(true);
  });

  it("falls back to comparing Origin against Host when Sec-Fetch-Site is absent", () => {
    expect(
      isCrossSiteRequest(req({ origin: "https://app.test", host: "app.test" })),
    ).toBe(false);
    expect(
      isCrossSiteRequest(req({ origin: "https://evil.test", host: "app.test" })),
    ).toBe(true);
  });

  it("treats an unparseable Origin as hostile", () => {
    expect(isCrossSiteRequest(req({ origin: "not-a-url", host: "app.test" }))).toBe(true);
  });

  it("allows requests with no browser indicators (server-to-server)", () => {
    // CSRF needs a browser to attach ambient cookies; without one there is no
    // vector, and rejecting would break legitimate server callers.
    expect(isCrossSiteRequest(req({}))).toBe(false);
  });

  it("prefers Sec-Fetch-Site over a spoofable Origin", () => {
    expect(
      isCrossSiteRequest(
        req({ "sec-fetch-site": "cross-site", origin: "https://app.test", host: "app.test" }),
      ),
    ).toBe(true);
  });
});

describe("rejectCrossSite", () => {
  it("returns null for same-origin so the handler proceeds", () => {
    expect(rejectCrossSite(req({ "sec-fetch-site": "same-origin" }))).toBeNull();
  });

  it("returns a typed 403 envelope for cross-site", async () => {
    const response = rejectCrossSite(req({ "sec-fetch-site": "cross-site" }));

    expect(response).not.toBeNull();
    expect(response?.status).toBe(403);
    expect(JSON.parse(await response!.text())).toEqual({
      success: false,
      error: {
        code: "CROSS_ORIGIN_FORBIDDEN",
        message: "Cross-origin request rejected.",
      },
    });
  });
});
