const AUTH_ERROR_MESSAGES = new Map([
  ["session_expired", "Your session has expired. Please sign in again."],
  [
    "account_exists_use_password",
    "An account with this email already exists. Sign in with your email and password.",
  ],
]);

/** Fixed text for a `?error=` code. The raw value is never shown: it is attacker-controlled. */
export function authErrorMessage(code: string): string {
  return AUTH_ERROR_MESSAGES.get(code) ?? "Sign-in failed. Please try again.";
}
