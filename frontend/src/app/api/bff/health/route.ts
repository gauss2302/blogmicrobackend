import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";

export async function GET(request: Request) {
  const { upstream } = await proxyGateway<Record<string, unknown>>(
    request,
    "/health",
    {
      method: "GET",
    },
  );

  if (!upstream.ok) {
    return toFailureResponse(
      upstream.status,
      "BACKEND_UNAVAILABLE",
      "API gateway is unavailable.",
    );
  }

  return toSuccessResponse({ healthy: true }, 200);
}
