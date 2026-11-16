import { proxyGateway, toFailureResponse, toSuccessResponse } from "@/lib/server/gateway";
import {
  accessTokenExpiresIn,
  authorizationHeader,
  readAccessToken,
} from "@/lib/server/session";

interface ValidateData {
  valid?: boolean;
  user_id?: string;
  email?: string;
}

/**
 * Restores the browser session from the access cookie without rotating the
 * refresh token.
 *
 * A full page load used to call /auth/refresh. After refresh reuse revokes the
 * whole token family, two tabs loading together would log the user out. This
 * route only asks the gateway whether the current access token is still valid.
 */
export async function GET(request: Request) {
  const authorization = authorizationHeader(request);
  const accessToken = readAccessToken(request);
  if (!authorization || !accessToken) {
    return toFailureResponse(401, "UNAUTHORIZED", "Authentication required.");
  }

  const { upstream, payload } = await proxyGateway<ValidateData>(
    request,
    "/api/v1/auth/validate",
    {
      method: "GET",
      headers: { authorization },
    },
  );

  const userId = payload?.data?.user_id;
  if (!payload?.success || !payload.data?.valid || !userId) {
    return toFailureResponse(
      upstream.status === 200 ? 401 : upstream.status,
      "UNAUTHORIZED",
      "Session is not valid.",
      payload?.error,
    );
  }

  return toSuccessResponse(
    {
      tokenType: "Bearer",
      expiresIn: accessTokenExpiresIn(accessToken),
      user: {
        id: userId,
        email: payload.data.email ?? "",
      },
    },
    200,
  );
}
