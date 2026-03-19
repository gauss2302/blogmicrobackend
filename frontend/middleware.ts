import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

import { AUTH_REFRESH_COOKIE_NAME } from "@/lib/auth/server-constants";

const protectedPrefix = "/app";

export function middleware(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  const hasRefreshCookie = Boolean(
    request.cookies.get(AUTH_REFRESH_COOKIE_NAME)?.value,
  );

  if (pathname.startsWith(protectedPrefix) && !hasRefreshCookie) {
    const loginURL = new URL("/auth/login", request.url);
    if (pathname !== "/app") {
      loginURL.searchParams.set("next", `${pathname}${search}`);
    }
    return NextResponse.redirect(loginURL);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/app/:path*", "/auth/login", "/auth/register"],
};
