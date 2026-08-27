import { z } from "zod";

const digestSchema = z.string().regex(/^[0-9a-f]{64}$/);
const availabilitySchema = z.enum([
  "available",
  "not_measured",
  "not_applicable",
  "unsupported",
  "not_authorized",
  "not_run",
  "withheld_private_parent",
]);
const stringListSchema = z.array(z.string());

const artifactSchema = z
  .object({
    kind: z.enum(["capsule_component", "release_file", "sidecar"]),
    id: z.string().min(1),
    schema_version: z.string().min(1),
    payload_sha256: digestSchema,
    artifact_digest: digestSchema,
  })
  .strict();

const countSchema = z
  .object({ name: z.string().min(1), value: z.number().int().nonnegative() })
  .strict();

const methodGenerationSchema = z
  .object({
    generation: z.string().min(1),
    state: z.string().min(1),
    evidence: z.array(artifactSchema),
    frozen_denominators: z.array(countSchema),
    observations: stringListSchema,
    non_claims: stringListSchema,
    guarding_tests: stringListSchema,
    deep_link: z.string().startsWith("#autopsy/method/"),
  })
  .strict();

const methodTransitionSchema = z
  .object({
    from: z.string().min(1),
    to: z.string().min(1),
    reason: z.string().min(1),
    evidence: z.array(artifactSchema),
    guarding_test: z.string().min(1),
    from_deep_link: z.string().startsWith("#autopsy/method/"),
    target_deep_link: z.string().startsWith("#autopsy/method/"),
  })
  .strict();

const transportLayerSchema = z
  .object({
    layer: z.string().min(1),
    status: z.string().min(1),
    object_digest: digestSchema.nullable(),
    object_digest_availability: availabilitySchema,
    required_fields: stringListSchema.nullable(),
    decisive_channels: stringListSchema.nullable(),
    detail_availability: availabilitySchema,
    deep_link: z.string().startsWith("#autopsy/transport/"),
  })
  .strict();

const transportPathSchema = z
  .object({
    path_id: z.string().min(1),
    label: z.string().min(1),
    terminal_state: z.string().min(1),
    evidence: artifactSchema,
    layers: z.array(transportLayerSchema),
    deep_link: z.string().startsWith("#autopsy/transport/"),
  })
  .strict();

const challengeReceiptSchema = z
  .object({
    schema_version: z.literal("evalwitness.claim-challenge-receipt.v1"),
    verifier: z.string().min(1),
    claim_id: z.string().min(1),
    source_capsule_id: digestSchema,
    source_manifest_digest: digestSchema,
    source_ledger_digest: digestSchema,
    challenge_id: z.string().min(1),
    class: z.string().min(1),
    mutation: z.string().min(1),
    expected_guard: z.string().min(1),
    observed_guard: z.string().min(1),
    before_state: z.string().min(1),
    after_state: z.string().min(1),
    sealed_source_digest: digestSchema,
    after_sealed_source_digest: digestSchema,
    passed: z.literal(true),
    digest: digestSchema,
    replay_command: z.string().startsWith("evalwitness claim challenge "),
  })
  .strict();

const challengeClassSchema = z
  .object({
    class: z.string().min(1),
    availability: availabilitySchema,
    global_receipt_count: z.number().int().nonnegative(),
    inapplicable_reason: z.string(),
    receipt: challengeReceiptSchema.nullable(),
  })
  .strict();

const ownerInspectionDispositionSchema = z
  .object({
    accepted: z.number().int().nonnegative(),
    revision_required: z.number().int().nonnegative(),
    unresolved: z.number().int().nonnegative(),
  })
  .strict();

const ownerInspectionSchema = z
  .object({
    availability: z.literal("available"),
    schema_version: z.literal("evalwitness.relation-owner-inspection-public-attestation.v1"),
    canonical_policy: z.literal("evalwitness.relation-canonical-json.v1"),
    evidence_kind: z.literal("owner_authorized_agent_assisted_construct_inspection"),
    inspection_mode: z.literal("private_chain_verified_public_aggregate"),
    inspection_date: z.string().min(1),
    package_inventory_digest: digestSchema,
    assessments: z
      .object({
        required: z.number().int().nonnegative(),
        journal_events: z.number().int().nonnegative(),
        corrections: z.number().int().nonnegative(),
        core: z.number().int().nonnegative(),
        scarcity_cases: z.number().int().nonnegative(),
        scarcity_boundary: z.number().int().nonnegative(),
        completed: z.number().int().nonnegative(),
        core_cases: z.number().int().nonnegative(),
        scarcity_case_count: z.number().int().nonnegative(),
        scarcity_test_cases: z.number().int().nonnegative(),
      })
      .strict(),
    dimensions: z.array(
      z
        .object({
          scope: z.string().min(1),
          dimension: z.string().min(1),
          applicable: z.number().int().nonnegative(),
          passed: z.number().int().nonnegative(),
          failed: z.number().int().nonnegative(),
          indeterminate: z.number().int().nonnegative(),
        })
        .strict(),
    ),
    outcomes: z
      .object({
        core: ownerInspectionDispositionSchema,
        scarcity_cases: ownerInspectionDispositionSchema,
        scarcity_boundary: z.string().min(1),
        core_status: z.string().min(1),
        scarcity_status: z.string().min(1),
        overall_status: z.string().min(1),
      })
      .strict(),
    human_study_status: z.literal("not_run"),
    external_action_status: z.literal("not_authorized"),
    disclosure: z
      .object({
        private_chain_verified: z.literal(true),
        private_journal_identities_disclosed: z.literal(false),
        restricted_evidence_disclosed: z.literal(false),
        public_validation_scope: z.literal(
          "schema_digest_denominators_statuses_and_claim_boundary",
        ),
        source_reproduction: z.literal("requires_withheld_restricted_owner_inputs"),
        signature_status: z.literal("unsigned_content_addressed"),
        capsule_status: z.literal("not_yet_capsule_bound"),
      })
      .strict(),
    claims: z.array(
      z
        .object({
          id: z.string().min(1),
          claim: z.string().min(1),
          status: z.enum([
            "supported",
            "owner_attested",
            "not_run",
            "not_authorized",
            "unsupported",
            "not_publicly_reproducible",
          ]),
          evidence: z.string().min(1),
        })
        .strict(),
    ),
    digest: digestSchema,
    source: artifactSchema,
  })
  .strict();

const stressReductionStepSchema = z
  .object({
    index: z.number().int().nonnegative(),
    unit_id: z.string().min(1),
    decision: z.enum(["accepted", "rejected"]),
    relation_revalidated: z.boolean(),
    privacy_revalidated: z.boolean(),
    violation_preserved: z.boolean(),
    before_digest: digestSchema,
    candidate_digest: digestSchema,
    after_digest: digestSchema,
    observation_digest: digestSchema,
  })
  .strict();

const stressWitnessLineSchema = z
  .object({
    trajectory_id: z.string().min(1),
    unit_id: z.string().min(1),
    fixture_path: z.string().min(1),
    line: z.number().int().positive(),
    content: z.string().min(1),
  })
  .strict();

const stressSchema = z
  .object({
    case_study_id: z.string().min(1),
    data_role: z.literal("adapter_development"),
    status: z.literal("mechanism_demonstration"),
    task_requirement: z.string().min(1),
    relation_id: z.string().min(1),
    relation_kind: z.string().min(1),
    relation_digest: digestSchema,
    control_arm_id: z.literal("zero-cost-first-listed"),
    original_order: z.array(z.string().min(1)).length(2),
    transformed_order: z.array(z.string().min(1)).length(2),
    original_selected: z.string().min(1),
    transformed_selected: z.string().min(1),
    outcome: z.literal("violated"),
    original_input_digest: digestSchema,
    reduced_input_digest: digestSchema,
    original_line_units: z.number().int().positive(),
    final_line_units: z.number().int().positive(),
    reduction_attempts: z.number().int().positive(),
    accepted_reductions: z.number().int().nonnegative(),
    rejected_reductions: z.number().int().nonnegative(),
    reduction_basis_points: z.number().int().nonnegative(),
    minimality: z.literal("one_minimal"),
    steps: z.array(stressReductionStepSchema),
    final_witness: z.array(stressWitnessLineSchema),
    empirical_units: z.literal(0),
    provider_calls: z.literal(0),
    network_required: z.literal(false),
    claim_boundary: z.string().min(1),
    allowed_claims: stringListSchema,
    forbidden_claims: stringListSchema,
    reproduction_command: z.string().min(1),
    digest: digestSchema,
    source: artifactSchema,
  })
  .strict();

const relianceEstimateSchema = z
  .object({
    estimate: z.string().min(1),
    standard_error: z.string().min(1),
    lower: z.string().min(1),
    upper: z.string().min(1),
    adjusted_p_value: z.string().min(1),
  })
  .strict();

const relianceOutcomeSchema = z
  .object({
    dimension_id: z.string().min(1),
    map_term_id: z.string().min(1),
    term_kind: z.enum(["main_effect", "interaction"]),
    factors: z.array(z.string().min(1)).min(1),
    outcome_id: z.string().min(1),
    status: z.enum(["measured", "not_measured", "unsupported"]),
    numerator: z.number().int().nonnegative(),
    denominator: z.number().int().positive(),
    invalid_cells: z.number().int().nonnegative(),
    estimate: relianceEstimateSchema.nullable().optional(),
    reason: z.string().optional(),
    unit: z.string().min(1),
    interval: z.string().min(1),
    policy: z.string().min(1),
    route: z.string().min(1),
    domain: z.string().min(1),
    data_role: z.string().min(1),
    evidence_level: z.string().min(1),
    capsule_id: digestSchema,
    capsule_expression: z.string().startsWith("/terms/"),
    source_analysis_digest: digestSchema,
    claim_boundary: z.string().min(1),
  })
  .strict();

const relianceSelectorBudgetSchema = z
  .object({
    budget_tokens: z.number().int().positive(),
    assignment_targets: z.number().int().positive(),
    exact_targets: z.number().int().nonnegative(),
    changed_targets: z.number().int().nonnegative(),
    unrendered_targets: z.number().int().nonnegative(),
    dropped_targets: z.number().int().nonnegative(),
    assigned_event_bytes: z.number().int().nonnegative(),
    retained_assigned_event_bytes: z.number().int().nonnegative(),
    assigned_rendered_bytes: z.number().int().nonnegative(),
    retained_assigned_rendered_bytes: z.number().int().nonnegative(),
    risk_flags: stringListSchema,
  })
  .strict();

const relianceSelectorSchema = z
  .object({
    term_id: z.string().min(1),
    factor_id: z.string().min(1),
    effect_status: z.enum([
      "adjusted_effect_detected",
      "no_adjusted_effect_detected",
      "analysis_inconclusive",
    ]),
    assignment_targets: z.number().int().positive(),
    minimum_event_score: z.number().int(),
    maximum_event_score: z.number().int(),
    budgets: z.array(relianceSelectorBudgetSchema).min(1),
  })
  .strict();

const relianceArmContrastSchema = z
  .object({
    comparison_digest: digestSchema,
    contrast_id: z.string().min(1),
    kind: z.enum(["evidence_policy", "entrypoint", "model_family", "provider", "route"]),
    reference_arm_id: z.string().min(1),
    comparator_arm_id: z.string().min(1),
    direction: z.literal("comparator_minus_reference"),
    changed_dimensions: z.array(z.string().min(1)).min(1),
    support: z.enum(["supported", "unsupported"]),
    reason: z.string().optional(),
    numerator: z.number().int().nonnegative(),
    denominator: z.number().int().nonnegative(),
    invalid_pairs: z.number().int().nonnegative(),
  })
  .strict();

const relianceWitnessSchema = z
  .object({
    case_id: z.string().min(1),
    relation_digest: digestSchema,
    witness_digest: digestSchema,
    final_evaluation_digest: digestSchema,
    source_analysis_digest: digestSchema,
    binding_status: z.literal("relation_level_not_factorial_cell_attributed"),
    original_input_digest: digestSchema,
    reduced_input_digest: digestSchema,
    final_units: z.number().int().positive(),
    evaluations: z.number().int().positive(),
    raw_trajectory_content_shown: z.literal(false),
  })
  .strict();

const profileEvidenceReportSchema = z
  .object({
    identity: z.string().min(1),
    summary: z.string().min(1),
    dimensions: z.number().int().nonnegative(),
    digest: digestSchema,
    text: z.string().min(1),
    markdown: z.string().min(1),
    claim_check_exprs: z.array(z.string().min(1)),
    otel: z
      .object({
        name: z.string().min(1),
        trace_id: z.string().min(1),
        profile_digest: digestSchema,
        route_scope: z.string().min(1),
      })
      .strict(),
    dimensions_json: z.array(z.unknown()),
  })
  .strict();

const profileExplorerViewSchema = z
  .object({
    report: profileEvidenceReportSchema,
    markdown: z.string().min(1),
  })
  .strict();

const relianceSchema = z
  .object({
    schema_version: z.literal("evalwitness.reliance-explorer-projection.v1"),
    availability: z.literal("available"),
    base_capsule_id: digestSchema,
    capsule_id: digestSchema,
    manifest_digest: digestSchema,
    map_digest: digestSchema,
    ledger_digest: digestSchema,
    profile_digest: digestSchema,
    paper_digest: digestSchema,
    source: artifactSchema,
    scope: z
      .object({
        evidence_role: z.literal("local_mechanism"),
        data_role: z.literal("mechanism_fixture"),
        evidence_level: z.literal("E1"),
        domain: z.literal("coding_agent_trajectory"),
        study_manifest_digest: digestSchema,
        entrypoint: z.string().min(1),
        criterion_id: z.string().min(1),
        score_tag: z.string().min(1),
        evidence_policy: z.string().min(1),
        provider_id: z.string().min(1),
        route_id: z.string().startsWith("route-"),
        requested_model: z.string().min(1),
        empirical: z.literal(false),
      })
      .strict(),
    source_tasks: z.number().int().positive(),
    registered_cells: z.number().int().positive(),
    outcome_bearing_cells: z.number().int().nonnegative(),
    excluded_cells: z.number().int().nonnegative(),
    completed_logical_calls: z.number().int().nonnegative(),
    cell_status_counts: z.array(
      z.object({ status: z.string().min(1), cells: z.number().int().nonnegative() }).strict(),
    ),
    profile_status_counts: z.array(
      z.object({ status: z.string().min(1), dimensions: z.number().int().nonnegative() }).strict(),
    ),
    outcomes: z.array(relianceOutcomeSchema).min(1),
    selectors: z.array(relianceSelectorSchema).min(1),
    arm_contrasts: z.array(relianceArmContrastSchema).min(5),
    witnesses: z.array(relianceWitnessSchema).min(1),
    allowed_claims: stringListSchema,
    forbidden_claims: stringListSchema,
    limitations: stringListSchema,
    current_claim_ids: stringListSchema,
    unsupported_claim_ids: stringListSchema,
    global_score_prohibited: z.literal(true),
    provider_calls: z.literal(0),
    network_required: z.literal(false),
    digest: digestSchema,
  })
  .strict();

const identicalResponseDecisionSchema = z
  .object({
    state: z.enum(["selected", "tied", "missing"]),
    selected_index: z.number().int().nonnegative().optional(),
    margin: z.string().min(1),
  })
  .strict();

const identicalResponseSchema = z
  .object({
    schema_version: z.literal("evalwitness.identical-response-explorer.v1"),
    availability: z.literal("available"),
    capsule_id: digestSchema,
    manifest_digest: digestSchema,
    base_capsule_id: digestSchema,
    ledger_digest: digestSchema,
    evidence_ceiling: z.literal("record_complete_external_parents_resolved"),
    status: z.literal("complete"),
    method: z.literal("distribution_aware_vs_chosen_token on the same immutable completion"),
    route_id: z.string().startsWith("route-"),
    model: z.string().min(1),
    observed_calls: z.number().int().positive(),
    comparison: z
      .object({
        task_groups: z.number().int().positive(),
        complete: z.number().int().nonnegative(),
        agreements: z.number().int().nonnegative(),
        disagreements: z.number().int().nonnegative(),
        unresolved: z.number().int().nonnegative(),
        disagreement_rate: z.string().min(1),
        outcome_covered: z.number().int().nonnegative(),
        outcome_coverage_rate: z.string().min(1),
        missingness_upper_bound: z.string().min(1),
      })
      .strict(),
    claims: z.array(
      z
        .object({
          claim_id: z.string().min(1),
          statement: z.string().min(1),
          status: z.string().min(1),
          evidence_level: z.string().min(1),
          evidence_components: stringListSchema,
          caveats: stringListSchema,
          challenge_receipts: z.number().int().nonnegative(),
        })
        .strict(),
    ),
    failures: z.array(
      z
        .object({
          group_id: z.string().min(1),
          task_id: z.string().min(1),
          response_digest: digestSchema,
          record_digest: digestSchema,
          distribution_aware: identicalResponseDecisionSchema,
          chosen_token: identicalResponseDecisionSchema,
        })
        .strict(),
    ),
    challenges: z
      .object({
        pack_digest: digestSchema,
        total: z.number().int().positive(),
        class_counts: z.array(countSchema),
        receipts: z.array(
          z
            .object({
              claim_id: z.string().min(1),
              challenge_id: z.string().min(1),
              class: z.string().min(1),
              mutation: z.string().min(1),
              observed_guard: z.string().min(1),
              before_state: z.string().min(1),
              after_state: z.string().min(1),
              digest: digestSchema,
              passed: z.literal(true),
            })
            .strict(),
        ),
      })
      .strict(),
    limitations: stringListSchema,
    reproduction: z
      .object({
        command: z.literal("scripts/evals/reproduce-identical-response-v5.sh"),
        status: z.literal("passed"),
        source_commit: z.string().regex(/^[0-9a-f]{40}$/),
        clean_clone_proof: z.literal(true),
        network: z.literal("denied"),
        provider_calls: z.literal(0),
        artifacts_verified: z.number().int().positive(),
        source: artifactSchema,
      })
      .strict(),
    source: artifactSchema,
    analysis_source: artifactSchema,
    digest: digestSchema,
  })
  .strict();

export const reportSchema = z
  .object({
    schema_version: z.literal("evalwitness.evidence-explorer-report.v2"),
    canonical_policy: z.literal("evalwitness.evidence-explorer-canonical-json.v1"),
    capsule: z
      .object({
        capsule_id: digestSchema,
        manifest_digest: digestSchema,
        ledger_digest: digestSchema,
        autopsy_digest: digestSchema,
        ledger_source: artifactSchema,
        autopsy_source: artifactSchema,
      })
      .strict(),
    scope: z
      .object({
        evaluator: z.string().min(1),
        route: z.string().nullable(),
        route_availability: availabilitySchema,
        evidence_role: z.string().min(1),
        release_version: z.string().min(1),
        development_fixtures: z.number().int().nonnegative(),
        empirical_task_groups: z.number().int().nonnegative(),
        research_admitted_sources: z.number().int().nonnegative(),
        provider_calls: z.number().int().nonnegative(),
        empirical: z.boolean(),
        provider_ranking: z.boolean(),
        restricted_material_excluded: z.boolean(),
      })
      .strict(),
    claim: z
      .object({
        claim_id: z.string().min(1),
        failable_property: z.string().min(1),
        observable_failure_condition: z.string().min(1),
        accepted: z.boolean(),
        claim_boundary: stringListSchema,
        known_limitations: stringListSchema,
        out_of_scope_uses: stringListSchema,
        source: artifactSchema,
      })
      .strict(),
    bom: z
      .object({
        bom_id: z.string().min(1),
        candidate_id: z.string().min(1),
        executable: z.string().min(1),
        operands: stringListSchema,
        required_fields: stringListSchema,
        decisive_channels: stringListSchema,
        surviving_channels: stringListSchema,
        truncated_required_fields: stringListSchema.nullable(),
        repository_state_digest: digestSchema,
        freshness: z
          .object({
            state: z.string().min(1),
            observed_state_digest: digestSchema,
            valid_from: z.string().min(1),
            valid_until: z.string().nullable(),
            evaluated_at: z.string().min(1),
            invalidation_edges: z.number().int().nonnegative(),
          })
          .strict(),
        source: artifactSchema,
      })
      .strict(),
    method: z
      .object({
        current: z.string().min(1),
        boundary: z.string().min(1),
        generations: z.array(methodGenerationSchema),
        transitions: z.array(methodTransitionSchema),
      })
      .strict(),
    owner_inspection: ownerInspectionSchema,
    transport: z
      .object({
        claim_id: z.string().min(1),
        accepted: z.boolean(),
        selected_path: z.string().min(1),
        selected_layer: z.string().min(1),
        paths: z.array(transportPathSchema),
      })
      .strict(),
    loss: z
      .object({
        certificate_id: z.string().min(1),
        path_id: z.string().min(1),
        first_loss_state: z.string().min(1),
        disposition: z.string().min(1),
        precedence: z.number().int().nonnegative(),
        failability_resolution: z.string().min(1),
        failability_reason: z.string().min(1),
        public_safe: z.boolean(),
        restricted_content: z.boolean(),
        supported_claim: z.string().min(1),
        unsupported_claims: stringListSchema,
        source: artifactSchema,
      })
      .strict(),
    stress: stressSchema,
    reliance: relianceSchema.optional(),
    profile: profileExplorerViewSchema.optional(),
    identical_response: identicalResponseSchema.optional(),
    dataset: z
      .object({
        dataset_id: z.string().min(1),
        title: z.string().min(1),
        evidence_role: z.string().min(1),
        counts: z.array(countSchema),
        known_limitations: stringListSchema,
        out_of_scope_uses: stringListSchema,
        source: artifactSchema,
      })
      .strict(),
    limitations: z.array(
      z
        .object({
          id: z.string().min(1),
          artifact_status: z.string().min(1),
          current_availability: availabilitySchema,
          boundary: z.string().min(1),
          required_resolution: z.string().min(1),
          source: artifactSchema,
          resolved_by: artifactSchema.nullable(),
        })
        .strict(),
    ),
    release: z
      .object({
        release_id: z.string().min(1),
        version: z.string().min(1),
        files: z.number().int().positive(),
        files_verified: z.number().int().positive(),
        all_files_verified: z.boolean(),
        public_projection: z.boolean(),
        restricted_material_excluded: z.boolean(),
        manifest: z.array(
          z
            .object({
              path: z.string().min(1),
              role: z.string().min(1),
              bytes: z.number().int().positive(),
              payload_sha256: digestSchema,
            })
            .strict(),
        ),
        source: artifactSchema,
      })
      .strict(),
    challenge: z
      .object({
        claim_id: z.string().min(1),
        claim_text: z.string().min(1),
        claim_status: z.string().min(1),
        evidence_level: z.string().min(1),
        total_receipts: z.number().int().positive(),
        selected_receipts: z.number().int().positive(),
        source_capsule_id: digestSchema,
        source_manifest_digest: digestSchema,
        source_ledger_digest: digestSchema,
        pack_digest: digestSchema,
        classes: z.array(challengeClassSchema),
        source: artifactSchema,
      })
      .strict(),
    extensions: z.array(
      z
        .object({
          extension_id: z.string().min(1),
          owner_task: z.string().min(1),
          required_types: stringListSchema,
          components: z.array(
            z.object({ type_id: z.string().min(1), component_id: digestSchema }).strict(),
          ),
          missing_types: stringListSchema,
          availability: availabilitySchema,
        })
        .strict(),
    ),
    reproduction: z
      .object({
        command: z.string().min(1),
        network_required: z.boolean(),
        provider_calls: z.number().int().nonnegative(),
        source: artifactSchema,
      })
      .strict(),
    digest: digestSchema,
  })
  .strict();

export type EvidenceReport = z.infer<typeof reportSchema>;
export type IdenticalResponseView = z.infer<typeof identicalResponseSchema>;
export type ProfileExplorerView = z.infer<typeof profileExplorerViewSchema>;
export type ArtifactRef = z.infer<typeof artifactSchema>;
export type Availability = z.infer<typeof availabilitySchema>;
export type ChallengeClass = z.infer<typeof challengeClassSchema>;
export type ChallengeReceipt = z.infer<typeof challengeReceiptSchema>;
export type MethodGeneration = z.infer<typeof methodGenerationSchema>;
export type RelianceArmContrast = z.infer<typeof relianceArmContrastSchema>;
export type RelianceOutcome = z.infer<typeof relianceOutcomeSchema>;
export type RelianceSelector = z.infer<typeof relianceSelectorSchema>;
export type RelianceView = z.infer<typeof relianceSchema>;
export type StressReductionStep = z.infer<typeof stressReductionStepSchema>;
export type StressView = z.infer<typeof stressSchema>;
export type TransportLayer = z.infer<typeof transportLayerSchema>;
export type TransportPath = z.infer<typeof transportPathSchema>;

export async function loadEmbeddedReport(): Promise<EvidenceReport> {
  const encoded = readMetaValue("evalwitness-report");
  const expectedDigest = readDigestMeta("evalwitness-report-sha256");
  const raw = decodeBase64(encoded);
  const actualDigest = await sha256(raw);
  if (actualDigest !== expectedDigest) {
    throw new Error("The embedded report bytes do not match the renderer binding.");
  }
  return reportSchema.parse(JSON.parse(new TextDecoder().decode(raw)));
}

function readMetaValue(name: string): string {
  const element = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`);
  const value = element?.content.trim();
  if (value === undefined || value === "") {
    throw new Error(`The rendered ${name} value is missing.`);
  }
  return value;
}

function readDigestMeta(name: string): string {
  const value = readMetaValue(name);
  if (!/^[0-9a-f]{64}$/.test(value)) {
    throw new Error(`The rendered ${name} value is invalid.`);
  }
  return value;
}

function decodeBase64(encoded: string): Uint8Array {
  const decoded = atob(encoded);
  return Uint8Array.from(decoded, (character) => character.charCodeAt(0));
}

async function sha256(raw: Uint8Array): Promise<string> {
  if (crypto.subtle === undefined) {
    throw new Error("This browser cannot verify the embedded report binding.");
  }
  const buffer = new ArrayBuffer(raw.byteLength);
  new Uint8Array(buffer).set(raw);
  const digest = await crypto.subtle.digest("SHA-256", buffer);
  return Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, "0")).join(
    "",
  );
}
