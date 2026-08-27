import type { RelianceOutcome, RelianceSelector, RelianceView } from "@/lib/report";

export const relianceOutcomeOptions = [
  { id: "visible_probability_mass", label: "Visible mass" },
  { id: "conditional_expected_score", label: "Expected score" },
  { id: "decision_flip", label: "Decision flip" },
  { id: "abstention_transition", label: "Abstention" },
] as const;

export type RelianceOutcomeID = (typeof relianceOutcomeOptions)[number]["id"];
export type RelianceTermKind = RelianceOutcome["term_kind"];

export function selectRelianceOutcomes(
  view: RelianceView,
  outcomeID: RelianceOutcomeID,
  termKind: RelianceTermKind,
): RelianceOutcome[] {
  return view.outcomes.filter(
    (outcome) => outcome.outcome_id === outcomeID && outcome.term_kind === termKind,
  );
}

export function relianceEffectScale(outcomes: RelianceOutcome[]): number {
  const magnitudes = outcomes.flatMap((outcome) => {
    if (outcome.estimate === undefined || outcome.estimate === null) {
      return [];
    }
    return [
      Math.abs(Number(outcome.estimate.lower)),
      Math.abs(Number(outcome.estimate.upper)),
      Math.abs(Number(outcome.estimate.estimate)),
    ];
  });
  return Math.max(...magnitudes, 1e-12);
}

export function relianceBarGeometry(
  outcome: RelianceOutcome,
  scale: number,
): { left: number; width: number; positive: boolean } | null {
  if (outcome.estimate === undefined || outcome.estimate === null || scale <= 0) {
    return null;
  }
  const estimate = Number(outcome.estimate.estimate);
  const width = Math.min(50, (Math.abs(estimate) / scale) * 50);
  return { left: estimate < 0 ? 50 - width : 50, width, positive: estimate >= 0 };
}

export function relianceIntervalGeometry(
  outcome: RelianceOutcome,
  scale: number,
): { lower: number; point: number; upper: number } | null {
  if (outcome.estimate === undefined || outcome.estimate === null || scale <= 0) {
    return null;
  }
  return {
    lower: effectCoordinate(Number(outcome.estimate.lower), scale),
    point: effectCoordinate(Number(outcome.estimate.estimate), scale),
    upper: effectCoordinate(Number(outcome.estimate.upper), scale),
  };
}

function effectCoordinate(value: number, scale: number): number {
  return Math.max(5, Math.min(195, 100 + (value / scale) * 95));
}

export function selectorForOutcome(
  view: RelianceView,
  outcome: RelianceOutcome,
): RelianceSelector | null {
  if (outcome.term_kind !== "main_effect" || outcome.factors.length !== 1) {
    return null;
  }
  return view.selectors.find((selector) => selector.factor_id === outcome.factors[0]) ?? null;
}

export function ratioPercent(numerator: number, denominator: number): string {
  if (denominator <= 0) {
    return "0%";
  }
  return `${((numerator / denominator) * 100).toFixed(1)}%`;
}

export function signedDecimal(value: string): string {
  const parsed = Number(value);
  return `${parsed > 0 ? "+" : ""}${parsed.toFixed(4)}`;
}
