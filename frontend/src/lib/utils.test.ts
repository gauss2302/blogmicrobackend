import { describe, expect, it } from "vitest";

import { formatDateTime } from "@/lib/utils";

describe("formatDateTime", () => {
  it("formats an ISO timestamp instead of echoing it", () => {
    const iso = "2026-11-27T09:30:00Z";
    const formatted = formatDateTime(iso);
    expect(formatted).not.toBe(iso);
    expect(formatted).toMatch(/2026/);
  });

  it("returns an unparseable value unchanged", () => {
    expect(formatDateTime("not a date")).toBe("not a date");
  });
});
