import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function POST(request: Request) {
  const authorization = request.headers.get("authorization");
  if (!authorization) {
    return toFailureResponse(
      401,
      "UNAUTHORIZED",
      "Authorization header is required for logout.",
    );
  }

  const { upstream, payload, setCookie } = await proxyGateway<{ loggedOut?: boolean }>(
    request,
    "/api/v1/auth/logout",
    {
      method: "POST",
      headers: {
        authorization,
      },
      body: JSON.stringify({}),
    },
  );

  if (!payload?.success) {
    return toFailureResponse(
      upstream.status,
      "LOGOUT_FAILED",
      "Logout failed.",
      payload?.error,
    );
  }

  return toSuccessResponse({ loggedOut: true }, upstream.status, setCookie);
}
