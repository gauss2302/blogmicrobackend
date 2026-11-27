import { describe, expect, it } from "vitest";

import { authErrorMessage } from "@/lib/auth/error-messages";

describe("authErrorMessage", () => {
  it("maps a known code to its text", () => {
    expect(authErrorMessage("session_expired")).toBe("Your session has expired. Please sign in again.");
  });

  it.each(["Call +1-555-0100 to restore access", "constructor", "access_denied"])(
    "never echoes an unknown code: %s",
    (code) => {
      expect(authErrorMessage(code)).toBe("Sign-in failed. Please try again.");
    },
  );
});
