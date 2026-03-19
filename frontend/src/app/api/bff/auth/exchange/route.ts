import { exchangeAuthCodeSchema } from "@/lib/auth/schemas";
import { mapSessionPayload, type BackendAuthPayload } from "@/lib/server/auth-mapper";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function POST(request: Request) {
  const rawBody = await request.json().catch(() => null);
  const parsed = exchangeAuthCodeSchema.safeParse(rawBody);
  if (!parsed.success) {
    return toFailureResponse(
      400,
      "INVALID_REQUEST",
      parsed.error.issues[0]?.message || "Invalid auth code payload.",
    );
  }

  const { upstream, payload, setCookie } = await proxyGateway<BackendAuthPayload>(
    request,
    "/api/v1/auth/exchange",
    {
      method: "POST",
      body: JSON.stringify({
        auth_code: parsed.data.authCode,
        code_verifier: parsed.data.codeVerifier,
      }),
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "EXCHANGE_FAILED",
      "Auth code exchange failed.",
      payload?.error,
    );
  }

  return toSuccessResponse(mapSessionPayload(payload.data), upstream.status, setCookie);
}
