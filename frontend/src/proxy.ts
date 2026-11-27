import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

import { mayHaveSession } from "@/lib/server/session";

const protectedPrefix = "/app";

// Builds the Content-Security-Policy. script-src uses a per-request nonce instead
// of 'unsafe-inline', so an injected inline <script> cannot execute. 'self' is
// kept so same-origin bundle chunks (/_next/static/...) still load. style-src
// keeps 'unsafe-inline' for Tailwind/framer-motion runtime styles. Development
// alone adds 'unsafe-eval': React uses eval() there for debugging (e.g.
// rebuilding server stacks in the error overlay). Production never gets it.
export function buildCsp(
  nonce: string,
  isDev = process.env.NODE_ENV === "development",
): string {
  return [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}'${isDev ? " 'unsafe-eval'" : ""}`,
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "font-src 'self' https://fonts.gstatic.com data:",
    "img-src 'self' data: https:",
    "connect-src 'self'",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "object-src 'none'",
  ].join("; ");
}

function generateNonce(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return btoa(String.fromCharCode(...bytes));
}

export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;

  // Redirect unauthenticated users away from the protected app shell. Checks
  // the Lax session hint: the Strict session cookies are missing when the user
  // follows a link from another site.
  if (pathname.startsWith(protectedPrefix) && !mayHaveSession(request.cookies)) {
    const loginURL = new URL("/auth/login", request.url);
    if (pathname !== "/app") {
      loginURL.searchParams.set("next", `${pathname}${search}`);
    }
    return NextResponse.redirect(loginURL);
  }

  // Per-request CSP nonce. Setting the CSP on the request headers lets Next.js
  // apply the same nonce to the framework's own inline scripts; the layout reads
  // x-nonce to apply it to the theme bootstrap script.
  const nonce = generateNonce();
  const csp = buildCsp(nonce);

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("content-security-policy", csp);

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("content-security-policy", csp);
  return response;
}

export const config = {
  // Run on all routes except API, Next static assets, and the favicon, and skip
  // prefetches so the nonce'd document is generated for real navigations.
  matcher: [
    {
      source: "/((?!api|_next/static|_next/image|favicon.ico).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
