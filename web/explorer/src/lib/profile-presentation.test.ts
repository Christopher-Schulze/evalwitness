import { describe, expect, it } from "vitest";

import { parseProfileDimensions } from "@/lib/profile-presentation";

describe("parseProfileDimensions", () => {
  it("sorts valid rows by id and keeps optional metric only when string", () => {
    const rows = parseProfileDimensions([
      { id: "transfer", status: "not_measured", scope: "terminal" },
      { id: "calibration", status: "measured", metric: "0.12", scope: "terminal" },
    ]);
    expect(rows).toEqual([
      { id: "calibration", metric: "0.12", scope: "terminal", status: "measured" },
      { id: "transfer", scope: "terminal", status: "not_measured" },
    ]);
  });

  it("drops non-object entries and entries without string id or status", () => {
    expect(
      parseProfileDimensions([
        null,
        42,
        { status: "measured" },
        { id: "x" },
        { id: 7, status: "failed" },
      ]),
    ).toEqual([]);
  });

  it("returns empty for non-array payloads", () => {
    expect(parseProfileDimensions(undefined)).toEqual([]);
    expect(parseProfileDimensions({})).toEqual([]);
    expect(parseProfileDimensions("nope")).toEqual([]);
  });

  it("coerces missing or non-string scope to empty string", () => {
    const rows = parseProfileDimensions([{ id: "a", status: "failed", scope: 9 }]);
    expect(rows).toEqual([{ id: "a", scope: "", status: "failed" }]);
  });
});
