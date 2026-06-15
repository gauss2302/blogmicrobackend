import { NextResponse } from "next/server";

import type { APIFailure, APISuccess } from "@/lib/auth/types";
import { BACKEND_API_URL } from "@/lib/auth/server-constants";

export interface GatewayError {
  code: string;
  message: string;
}

export interface GatewayEnvelope<T> {
  success: boolean;
  message: string;
  data?: T;
  error?: GatewayError;
}

export async function proxyGateway<T>(
  request: Request,
  path: string,
  init: RequestInit = {},
) {
  const headers = new Headers(init.headers);
  headers.set("accept", "application/json");

  if (init.body && !headers.has("content-type")) {
    headers.set("content-type", "application/json");
  }

  const cookie = request.headers.get("cookie");
  if (cookie) {
    headers.set("cookie", cookie);
  }

  const upstream = await fetch(`${BACKEND_API_URL}${path}`, {
    ...init,
    headers,
    cache: "no-store",
  });

  let payload: GatewayEnvelope<T> | null = null;
  try {
    payload = (await upstream.json()) as GatewayEnvelope<T>;
  } catch {
    payload = null;
  }

  return {
    upstream,
    payload,
    // getSetCookie() returns each Set-Cookie as its own entry; Headers.get()
    // would collapse multiple cookies into one comma-joined (corrupt) value.
    setCookies: upstream.headers.getSetCookie(),
  };
}

export function toSuccessResponse<T>(
  data: T,
  status = 200,
  setCookies?: string[] | null,
) {
  const response = NextResponse.json<APISuccess<T>>(
    {
      success: true,
      data,
    },
    { status },
  );

  applyUpstreamCookies(response, setCookies);

  return response;
}

interface ParsedCookie {
  name: string;
  value: string;
  path?: string;
  maxAge?: number;
  expires?: Date;
}

// Re-issue cookies set by the upstream gateway with hardened attributes instead
// of relaying them verbatim. The BFF→gateway hop is plain HTTP, so the upstream
// Set-Cookie may lack Secure and is not pinned to SameSite=Strict. Each cookie
// is forced HttpOnly + SameSite=Strict (+ Secure in production) while preserving
// its value, path and expiry so logout's deletion cookie (Max-Age=0) still clears.
function applyUpstreamCookies(
  response: NextResponse,
  setCookies?: string[] | null,
) {
  if (!setCookies?.length) return;
  for (const raw of setCookies) {
    const parsed = parseSetCookie(raw);
    if (!parsed) continue;
    response.cookies.set(parsed.name, parsed.value, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "strict",
      path: parsed.path ?? "/",
      ...(parsed.maxAge !== undefined ? { maxAge: parsed.maxAge } : {}),
      ...(parsed.expires !== undefined ? { expires: parsed.expires } : {}),
    });
  }
}

function parseSetCookie(raw: string): ParsedCookie | null {
  const segments = raw.split(";");
  const first = segments.shift();
  if (!first) return null;
  const eq = first.indexOf("=");
  if (eq < 0) return null;
  const name = first.slice(0, eq).trim();
  const value = first.slice(eq + 1).trim();
  if (!name) return null;

  const parsed: ParsedCookie = { name, value };
  for (const segment of segments) {
    const sepIndex = segment.indexOf("=");
    const key = (sepIndex < 0 ? segment : segment.slice(0, sepIndex))
      .trim()
      .toLowerCase();
    const val = sepIndex < 0 ? "" : segment.slice(sepIndex + 1).trim();
    if (key === "path") {
      parsed.path = val;
    } else if (key === "max-age") {
      const n = Number(val);
      if (!Number.isNaN(n)) parsed.maxAge = n;
    } else if (key === "expires") {
      const d = new Date(val);
      if (!Number.isNaN(d.getTime())) parsed.expires = d;
    }
  }
  return parsed;
}

export function toFailureResponse(
  status: number,
  fallbackCode: string,
  fallbackMessage: string,
  upstreamError?: GatewayError,
) {
  return NextResponse.json<APIFailure>(
    {
      success: false,
      error: {
        code: upstreamError?.code || fallbackCode,
        message: upstreamError?.message || fallbackMessage,
      },
    },
    { status },
  );
}
