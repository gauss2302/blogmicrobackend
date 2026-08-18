import { NextResponse } from "next/server";

import type { APIFailure } from "@/lib/auth/types";

/**
 * CSRF protection for cookie-authenticated BFF routes.
 *
 * Every BFF route now authenticates via ambient cookies the browser attaches
 * automatically — the refresh token and, since the access token moved
 * server-side, the access cookie too. That makes *all* state-changing routes
 * CSRF-reachable, not just the auth ones: previously a mutation additionally
 * required an `Authorization` header, and a cross-site form cannot set custom
 * headers, which incidentally blocked CSRF. That incidental protection is gone,
 * so the check has to be explicit.
 *
 * The session cookies are `SameSite=Strict`, which already stops cross-site
 * sends in current browsers. This is the second layer: it also covers
 * same-site-but-different-origin callers (e.g. another subdomain) that
 * SameSite treats as trusted.
 */

/**
 * Returns true when the request originates from a different site and should be
 * rejected on a state-changing, cookie-authenticated endpoint.
 */
export function isCrossSiteRequest(request: Request): boolean {
  // Sec-Fetch-Site is sent by all modern browsers and is the most reliable signal.
  const secFetchSite = request.headers.get("sec-fetch-site");
  if (secFetchSite) {
    // "same-origin" = our own SPA; "none" = a direct user action (e.g. address bar).
    return secFetchSite !== "same-origin" && secFetchSite !== "none";
  }

  // Fallback for clients that omit Sec-Fetch-Site: compare Origin to the host.
  const origin = request.headers.get("origin");
  if (origin) {
    const host = request.headers.get("host");
    try {
      return new URL(origin).host !== host;
    } catch {
      return true;
    }
  }

  // No browser indicators at all (e.g. server-to-server): not a CSRF vector,
  // since CSRF requires a browser to attach ambient cookies.
  return false;
}

/**
 * Guard for state-changing handlers. Returns a 403 response to return early,
 * or null when the caller is same-origin.
 *
 * Usage keeps the check to one line per handler so it is hard to omit:
 *
 *   const blocked = rejectCrossSite(request);
 *   if (blocked) return blocked;
 */
export function rejectCrossSite(request: Request): NextResponse | null {
  if (!isCrossSiteRequest(request)) return null;
  return NextResponse.json<APIFailure>(
    {
      success: false,
      error: {
        code: "CROSS_ORIGIN_FORBIDDEN",
        message: "Cross-origin request rejected.",
      },
    },
    { status: 403 },
  );
}
