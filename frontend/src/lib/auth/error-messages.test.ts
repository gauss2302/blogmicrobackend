import { describe, expect, it } from "vitest";

import { authErrorMessage } from "@/lib/auth/error-messages";

describe("authErrorMessage", () => {
  it("maps a known code to its text", () => {
    expect(authErrorMessage("session_expired")).toBe("Your session has expired. Please sign in again.");
  });

  it("explains a Google sign-in refused for a password account", () => {
    expect(authErrorMessage("account_exists_use_password")).toBe(
      "An account with this email already exists. Sign in with your email and password.",
    );
  });

  it.each(["Call +1-555-0100 to restore access", "constructor", "access_denied", "<script>alert(1)</script>"])(
    "never echoes an unknown code: %s",
    (code) => {
      expect(authErrorMessage(code)).toBe("Sign-in failed. Please try again.");
    },
  );
});
