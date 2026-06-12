/**
 * CSRF protection for cookie-authenticated BFF routes.
 *
 * The auth BFF endpoints authenticate via the forwarded refresh-token cookie
 * (an ambient credential the browser attaches automatically). Without a same-origin
 * check, a malicious page could trigger them with the victim's cookies (e.g. force
 * a token refresh). These helpers reject cross-site requests.
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
