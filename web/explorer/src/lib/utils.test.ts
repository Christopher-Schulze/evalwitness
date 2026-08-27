import { describe, expect, it } from "vitest";

import { formatTimestamp, humanize, shortDigest } from "@/lib/utils";

describe("display utilities", () => {
  it("humanizes closed identifiers without changing their words", () => {
    expect(humanize("non-failable_counterexample")).toBe("non failable counterexample");
  });

  it("shortens but does not rewrite a digest", () => {
    const digest = "0123456789abcdef".repeat(4);
    expect(shortDigest(digest)).toBe("01234567…abcdef");
  });

  it("renders evidence timestamps in the declared UTC timezone", () => {
    expect(formatTimestamp("2026-08-10T15:00:05.09Z")).toContain("10 Aug 2026");
  });
});
