export interface ProfileDimensionPresentation {
  id: string;
  metric?: string;
  scope: string;
  status: string;
}

// parseProfileDimensions projects the profile report's dimensions_json payload
// into sorted, render-ready rows. Entries lacking a string id or status are
// dropped rather than rendered as empty cells.
export function parseProfileDimensions(raw: unknown): ProfileDimensionPresentation[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  const parsed: ProfileDimensionPresentation[] = [];
  for (const entry of raw) {
    if (typeof entry !== "object" || entry === null) {
      continue;
    }
    const record = entry as Record<string, unknown>;
    if (typeof record["id"] !== "string" || typeof record["status"] !== "string") {
      continue;
    }
    parsed.push({
      id: record["id"],
      ...(typeof record["metric"] === "string" ? { metric: record["metric"] } : {}),
      scope: typeof record["scope"] === "string" ? record["scope"] : "",
      status: record["status"],
    });
  }
  return parsed.sort((left, right) => left.id.localeCompare(right.id));
}
