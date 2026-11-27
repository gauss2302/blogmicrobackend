const AUTH_ERROR_MESSAGES = new Map([
  ["session_expired", "Your session has expired. Please sign in again."],
  ["too_many_attempts", "Too many sign-in attempts. Wait a few minutes and try again."],
  [
    "account_exists_use_password",
    "An account with this email already exists. Sign in with your email and password.",
  ],
]);

/** Fixed text for a `?error=` code. The raw value is never shown: it is attacker-controlled. */
export function authErrorMessage(code: string): string {
  return AUTH_ERROR_MESSAGES.get(code) ?? "Sign-in failed. Please try again.";
}

/**
 * Text for a failed password sign-in. A 429 (account lockout or rate limit)
 * gets fixed text from the map, never the server's message.
 */
export function loginErrorMessage(error: Error & { status?: number }): string {
  return error.status === 429 ? authErrorMessage("too_many_attempts") : error.message;
}
