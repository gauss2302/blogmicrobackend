import { registerSchema } from "@/lib/auth/schemas";
import { mapSessionPayload, type BackendAuthPayload } from "@/lib/server/auth-mapper";
import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function POST(request: Request) {
  const rawBody = await request.json().catch(() => null);
  const parsed = registerSchema.safeParse(rawBody);
  if (!parsed.success) {
    return toFailureResponse(
      400,
      "INVALID_REQUEST",
      parsed.error.issues[0]?.message || "Invalid registration payload.",
    );
  }

  const { upstream, payload, setCookie } = await proxyGateway<BackendAuthPayload>(
    request,
    "/api/v1/auth/register",
    {
      method: "POST",
      body: JSON.stringify(parsed.data),
    },
  );

  if (!payload?.success || !payload.data) {
    return toFailureResponse(
      upstream.status,
      "REGISTER_FAILED",
      "Registration failed.",
      payload?.error,
    );
  }

  return toSuccessResponse(mapSessionPayload(payload.data), upstream.status, setCookie);
}
