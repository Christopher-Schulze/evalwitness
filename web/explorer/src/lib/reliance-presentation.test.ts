import { describe, expect, it } from "vitest";

import {
  ratioPercent,
  relianceBarGeometry,
  relianceEffectScale,
  relianceIntervalGeometry,
  signedDecimal,
} from "@/lib/reliance-presentation";
import type { RelianceOutcome } from "@/lib/report";

function measuredOutcome(estimate: string, lower: string, upper: string): RelianceOutcome {
  return {
    dimension_id: "evidence_reliance.patch_edit.visible_probability_mass",
    map_term_id: "patch_edit",
    term_kind: "main_effect",
    factors: ["patch_edit"],
    outcome_id: "visible_probability_mass",
    status: "measured",
    numerator: 1536,
    denominator: 1536,
    invalid_cells: 0,
    estimate: { estimate, standard_error: "0.01", lower, upper, adjusted_p_value: "0.01" },
    unit: "source_task_clustered_cell_contrast",
    interval: "bonferroni_family_adjusted_two_sided_cluster_sandwich_interval",
    policy: "source_task_cluster_sandwich_ols.v1+bonferroni",
    route: "route-reference",
    domain: "coding_agent_trajectory",
    data_role: "mechanism_fixture",
    evidence_level: "E1",
    capsule_id: "a".repeat(64),
    capsule_expression: "/terms/0/outcomes/0",
    source_analysis_digest: "b".repeat(64),
    claim_boundary: "local_mechanism_only_no_transfer_or_agent_causality",
  };
}

describe("reliance presentation", () => {
  it("keeps signed effects centered on zero", () => {
    const negative = measuredOutcome("-0.25", "-0.4", "-0.1");
    const positive = measuredOutcome("0.5", "0.2", "0.8");
    const scale = relianceEffectScale([negative, positive]);

    expect(relianceBarGeometry(negative, scale)).toEqual({
      left: 34.375,
      width: 15.625,
      positive: false,
    });
    expect(relianceBarGeometry(positive, scale)).toEqual({
      left: 50,
      width: 31.25,
      positive: true,
    });
    expect(relianceIntervalGeometry(negative, scale)).toEqual({
      lower: 52.5,
      point: 70.3125,
      upper: 88.125,
    });
  });

  it("formats denominators and signed decimals without hiding zero", () => {
    expect(ratioPercent(3, 40)).toBe("7.5%");
    expect(ratioPercent(0, 0)).toBe("0%");
    expect(signedDecimal("0.0625")).toBe("+0.0625");
    expect(signedDecimal("-0.125")).toBe("-0.1250");
  });
});
