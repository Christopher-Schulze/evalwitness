import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { RelianceSection } from "@/components/reliance-section";
import type { RelianceArmContrast, RelianceOutcome, RelianceView } from "@/lib/report";

const digest = "a".repeat(64);

function outcome(
  id: string,
  outcomeID: string,
  termKind: RelianceOutcome["term_kind"],
  factors: string[],
): RelianceOutcome {
  return {
    dimension_id: `evidence_reliance.${id}.${outcomeID}`,
    map_term_id: id,
    term_kind: termKind,
    factors,
    outcome_id: outcomeID,
    status: "measured",
    numerator: 1536,
    denominator: 1536,
    invalid_cells: 0,
    estimate: {
      estimate: "0.0625",
      standard_error: "0.01",
      lower: "0.04",
      upper: "0.08",
      adjusted_p_value: "0.02",
    },
    unit: "source_task_clustered_cell_contrast",
    interval: "bonferroni_family_adjusted_two_sided_cluster_sandwich_interval",
    policy: "source_task_cluster_sandwich_ols.v1+bonferroni",
    route: "route-reference",
    domain: "coding_agent_trajectory",
    data_role: "mechanism_fixture",
    evidence_level: "E1",
    capsule_id: digest,
    capsule_expression: "/terms/0/outcomes/0",
    source_analysis_digest: digest,
    claim_boundary: "local_mechanism_only_no_transfer_or_agent_causality",
  };
}

function armContrasts(): RelianceArmContrast[] {
  const kinds: RelianceArmContrast["kind"][] = [
    "evidence_policy",
    "entrypoint",
    "model_family",
    "provider",
    "route",
  ];
  return kinds.map((kind) => ({
    comparison_digest: digest,
    contrast_id: kind,
    kind,
    reference_arm_id: "reference",
    comparator_arm_id: "comparator",
    direction: "comparator_minus_reference",
    changed_dimensions: [kind],
    support: "unsupported",
    reason: "confounded fixture arms",
    numerator: 0,
    denominator: 0,
    invalid_pairs: 0,
  }));
}

function relianceView(): RelianceView {
  const outcomeIDs = [
    "visible_probability_mass",
    "conditional_expected_score",
    "decision_flip",
    "abstention_transition",
  ];
  const outcomes = outcomeIDs.flatMap((outcomeID) => [
    outcome("patch_edit", outcomeID, "main_effect", ["patch_edit"]),
    outcome("patch_edit_x_test_result", outcomeID, "interaction", ["patch_edit", "test_result"]),
  ]);
  return {
    schema_version: "evalwitness.reliance-explorer-projection.v1",
    availability: "available",
    base_capsule_id: "b".repeat(64),
    capsule_id: digest,
    manifest_digest: digest,
    map_digest: digest,
    ledger_digest: digest,
    profile_digest: digest,
    paper_digest: digest,
    source: {
      kind: "capsule_component",
      id: digest,
      schema_version: "evalwitness.evidence-reliance-map.v1",
      payload_sha256: digest,
      artifact_digest: digest,
    },
    scope: {
      evidence_role: "local_mechanism",
      data_role: "mechanism_fixture",
      evidence_level: "E1",
      domain: "coding_agent_trajectory",
      study_manifest_digest: digest,
      entrypoint: "reference-entrypoint",
      criterion_id: "task_correctness",
      score_tag: "SCORE",
      evidence_policy: "explicit_verifier",
      provider_id: "reference-provider",
      route_id: "route-reference",
      requested_model: "reference-model",
      empirical: false,
    },
    source_tasks: 24,
    registered_cells: 1536,
    outcome_bearing_cells: 1536,
    excluded_cells: 0,
    completed_logical_calls: 65,
    cell_status_counts: [{ status: "measured", cells: 1536 }],
    profile_status_counts: [{ status: "measured", dimensions: outcomes.length }],
    outcomes,
    selectors: [
      {
        term_id: "patch_edit",
        factor_id: "patch_edit",
        effect_status: "adjusted_effect_detected",
        assignment_targets: 8,
        minimum_event_score: 1,
        maximum_event_score: 4,
        budgets: [
          {
            budget_tokens: 512,
            assignment_targets: 8,
            exact_targets: 5,
            changed_targets: 1,
            unrendered_targets: 1,
            dropped_targets: 1,
            assigned_event_bytes: 800,
            retained_assigned_event_bytes: 600,
            assigned_rendered_bytes: 700,
            retained_assigned_rendered_bytes: 500,
            risk_flags: ["adjusted_effect_detected_with_nonexact_or_unrendered_target"],
          },
        ],
      },
    ],
    arm_contrasts: armContrasts(),
    witnesses: [
      {
        case_id: "case-reference",
        relation_digest: digest,
        witness_digest: digest,
        final_evaluation_digest: digest,
        source_analysis_digest: digest,
        binding_status: "relation_level_not_factorial_cell_attributed",
        original_input_digest: digest,
        reduced_input_digest: digest,
        final_units: 2,
        evaluations: 6,
        raw_trajectory_content_shown: false,
      },
    ],
    allowed_claims: ["local reliance"],
    forbidden_claims: ["agent causality", "model internals", "population transfer"],
    limitations: ["mechanism fixture only"],
    current_claim_ids: ["CLM-035"],
    unsupported_claim_ids: ["CLM-043"],
    global_score_prohibited: true,
    provider_calls: 0,
    network_required: false,
    digest,
  };
}

describe("RelianceSection", () => {
  it("renders the complete evidence boundary without raw trajectory content", () => {
    const html = renderToStaticMarkup(
      <RelianceSection onInspect={() => undefined} view={relianceView()} />,
    );

    expect(html).toContain("What actually moves the verifier?");
    expect(html).toContain("1536/1536 eligible");
    expect(html).toContain("Selector retention");
    expect(html).toContain("Every prespecified arm stays visible.");
    expect(html).toContain("raw trajectory content shown=false");
    expect(html).not.toContain("private trajectory");
  });
});
