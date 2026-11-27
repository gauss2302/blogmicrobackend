const AUTH_ERROR_MESSAGES = new Map([
  ["session_expired", "Your session has expired. Please sign in again."],
]);

/** Fixed text for a `?error=` code. The raw value is never shown: it is attacker-controlled. */
export function authErrorMessage(code: string): string {
  return AUTH_ERROR_MESSAGES.get(code) ?? "Sign-in failed. Please try again.";
}
