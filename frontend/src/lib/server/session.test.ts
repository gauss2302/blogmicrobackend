import { describe, expect, it } from "vitest";
import { NextResponse } from "next/server";

import {
  AUTH_ACCESS_COOKIE_NAME,
  accessTokenExpiresIn,
  authorizationHeader,
  clearSessionCookies,
  readAccessToken,
  setAccessCookie,
  toSessionResponse,
} from "@/lib/server/session";

function requestWithCookie(cookie: string): Request {
  return new Request("https://app.test/api/bff/posts", {
    headers: { cookie },
  });
}

describe("readAccessToken", () => {
  it("reads the access cookie", () => {
    const request = requestWithCookie(`${AUTH_ACCESS_COOKIE_NAME}=abc123`);
    expect(readAccessToken(request)).toBe("abc123");
  });

  it("picks the right cookie when several are present", () => {
    const request = requestWithCookie(
      `theme=dark; ${AUTH_ACCESS_COOKIE_NAME}=abc123; refresh_token=zzz`,
    );
    expect(readAccessToken(request)).toBe("abc123");
  });

  it("is not fooled by a cookie whose name merely ends with the same suffix", () => {
    // "not_access_token" must not satisfy a lookup for "access_token".
    const request = requestWithCookie(`not_${AUTH_ACCESS_COOKIE_NAME}=evil`);
    expect(readAccessToken(request)).toBeNull();
  });

  it("decodes percent-encoded values", () => {
    const request = requestWithCookie(`${AUTH_ACCESS_COOKIE_NAME}=a%20b`);
    expect(readAccessToken(request)).toBe("a b");
  });

  it("returns null with no cookie header at all", () => {
    const request = new Request("https://app.test/api/bff/posts");
    expect(readAccessToken(request)).toBeNull();
  });

  it("returns null for an empty value rather than an empty string", () => {
    const request = requestWithCookie(`${AUTH_ACCESS_COOKIE_NAME}=`);
    expect(readAccessToken(request)).toBeNull();
  });
});

describe("authorizationHeader", () => {
  it("builds a Bearer header from the cookie", () => {
    const request = requestWithCookie(`${AUTH_ACCESS_COOKIE_NAME}=tok`);
    expect(authorizationHeader(request)).toBe("Bearer tok");
  });

  it("returns null when there is no session, so routes can 401", () => {
    const request = new Request("https://app.test/api/bff/posts");
    expect(authorizationHeader(request)).toBeNull();
  });

  it("ignores a client-supplied Authorization header", () => {
    // The whole point of moving the token server-side: a caller cannot forge a
    // session by setting the header itself.
    const request = new Request("https://app.test/api/bff/posts", {
      headers: { authorization: "Bearer forged" },
    });
    expect(authorizationHeader(request)).toBeNull();
  });
});

describe("setAccessCookie", () => {
  it("marks the cookie HttpOnly and SameSite=Strict", () => {
    const response = NextResponse.json({ ok: true });
    setAccessCookie(response, "tok", 900);

    const cookie = response.cookies.get(AUTH_ACCESS_COOKIE_NAME);
    expect(cookie?.value).toBe("tok");
    expect(cookie?.httpOnly).toBe(true);
    expect(cookie?.sameSite).toBe("strict");
  });

  it("expires the cookie slightly before the token to avoid a mid-action 401", () => {
    const response = NextResponse.json({ ok: true });
    setAccessCookie(response, "tok", 900);

    const maxAge = response.cookies.get(AUTH_ACCESS_COOKIE_NAME)?.maxAge;
    expect(maxAge).toBeLessThan(900);
    expect(maxAge).toBeGreaterThan(860);
  });

  it("never sets a non-positive Max-Age, which would delete the cookie", () => {
    const response = NextResponse.json({ ok: true });
    setAccessCookie(response, "tok", 5); // shorter than the skew

    expect(response.cookies.get(AUTH_ACCESS_COOKIE_NAME)?.maxAge).toBeGreaterThan(0);
  });
});

describe("clearSessionCookies", () => {
  it("expires both session cookies", () => {
    const response = NextResponse.json({ ok: true });
    clearSessionCookies(response);

    expect(response.cookies.get(AUTH_ACCESS_COOKIE_NAME)?.maxAge).toBe(0);
    expect(response.cookies.get("refresh_token")?.maxAge).toBe(0);
  });
});

describe("toSessionResponse", () => {
  const upstream = {
    access_token: "secret-access-token",
    refresh_token: "secret-refresh-token",
    token_type: "Bearer",
    expires_in: 900,
    user: { id: "u1", email: "u@example.test", name: "U" },
  };

  it("never puts the access token in the response body", async () => {
    const response = toSessionResponse(upstream, 200);
    const text = await response.text();

    expect(text).not.toContain("secret-access-token");
    expect(text).not.toContain("secret-refresh-token");
    expect(text).not.toContain("accessToken");
  });

  it("returns only what the browser legitimately needs", async () => {
    const response = toSessionResponse(upstream, 200);
    const body = JSON.parse(await response.text());

    expect(body).toEqual({
      success: true,
      data: {
        tokenType: "Bearer",
        expiresIn: 900,
        user: { id: "u1", email: "u@example.test", name: "U" },
      },
    });
  });

  it("stores the access token in the HttpOnly cookie instead", () => {
    const response = toSessionResponse(upstream, 200);
    const cookie = response.cookies.get(AUTH_ACCESS_COOKIE_NAME);

    expect(cookie?.value).toBe("secret-access-token");
    expect(cookie?.httpOnly).toBe(true);
  });
});

describe("accessTokenExpiresIn", () => {
  it("returns the seconds remaining until exp", () => {
    const payload = Buffer.from(JSON.stringify({ exp: 2_000 })).toString("base64url");
    expect(accessTokenExpiresIn(`h.${payload}.s`, 1_700)).toBe(300);
  });

  it("returns 0 when the token is already expired or unreadable", () => {
    expect(accessTokenExpiresIn("not-a-jwt", 1_700)).toBe(0);
    const payload = Buffer.from(JSON.stringify({ exp: 1_000 })).toString("base64url");
    expect(accessTokenExpiresIn(`h.${payload}.s`, 1_700)).toBe(0);
  });
});
