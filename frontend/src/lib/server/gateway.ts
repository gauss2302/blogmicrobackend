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
    setCookie: upstream.headers.get("set-cookie"),
  };
}

export function toSuccessResponse<T>(
  data: T,
  status = 200,
  setCookie?: string | null,
) {
  const response = NextResponse.json<APISuccess<T>>(
    {
      success: true,
      data,
    },
    { status },
  );

  if (setCookie) {
    response.headers.set("set-cookie", setCookie);
  }

  return response;
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
