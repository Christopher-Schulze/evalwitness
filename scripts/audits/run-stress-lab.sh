#!/usr/bin/env bash
# Provider-free stress catalog, public case study, and optional held-out readiness gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d /tmp/evalwitness-stress-lab.XXXXXX)
tmp_bin="$tmp_dir/evalwitness"
trap 'rm -rf "${tmp_dir}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness
mkdir -p "$tmp_dir/empty-home"

for suffix in first second; do
  "$tmp_bin" stress catalog > "$tmp_dir/catalog-$suffix.json"
  EVALWITNESS_API_KEY=must-not-be-read \
    DEEPSEEK_API_KEY=must-not-be-read \
    EVALWITNESS_BASE_URL=http://127.0.0.1:1 \
  "$tmp_bin" stress development-case-study --repository-root . --format json > "$tmp_dir/case-$suffix.json"
  "$tmp_bin" stress development-case-study --repository-root . --format markdown > "$tmp_dir/case-$suffix.md"
  "$tmp_bin" stress development-case-study --repository-root . --format svg > "$tmp_dir/case-$suffix.svg"
  "$tmp_bin" stress development-challenge --repository-root . > "$tmp_dir/challenge-$suffix.json"
  (
    cd "$tmp_dir"
    env -i PATH="$PATH" HOME="$tmp_dir/empty-home" \
      EVALWITNESS_API_KEY=must-not-be-read \
      DEEPSEEK_API_KEY=must-not-be-read \
      EVALWITNESS_BASE_URL=http://127.0.0.1:1 \
      "$tmp_bin" stress verify-development-challenge \
        --challenge "@$tmp_dir/challenge-$suffix.json" > "$tmp_dir/challenge-receipt-$suffix.json"
  )
done

cmp "$tmp_dir/catalog-first.json" "$tmp_dir/catalog-second.json"
cmp "$tmp_dir/case-first.json" "$tmp_dir/case-second.json"
cmp "$tmp_dir/case-first.md" "$tmp_dir/case-second.md"
cmp "$tmp_dir/case-first.svg" "$tmp_dir/case-second.svg"
cmp "$tmp_dir/challenge-first.json" "$tmp_dir/challenge-second.json"
cmp "$tmp_dir/challenge-receipt-first.json" "$tmp_dir/challenge-receipt-second.json"
cmp "$tmp_dir/case-first.json" eval/results/stress-development-case-study-v1.json
cmp "$tmp_dir/case-first.md" eval/results/stress-development-case-study-v1.md
cmp "$tmp_dir/case-first.svg" eval/results/stress-development-case-study-v1.svg
cmp "$tmp_dir/challenge-first.json" eval/results/stress-development-challenge-v1.json
cmp "$tmp_dir/challenge-receipt-first.json" eval/results/stress-development-challenge-receipt-v1.json

if rg -n '<script|<image|<metadata|href=' "$tmp_dir/case-first.svg"; then
  echo "Stress development case-study SVG contains an external or active payload." >&2
  exit 1
fi

"$tmp_bin" stress validate \
  --type development-case-study \
  --repository-root . \
  --document @eval/results/stress-development-case-study-v1.json > "$tmp_dir/case-validation.json"
"$tmp_bin" stress schema --type development-case-study > "$tmp_dir/case.schema.json"
"$tmp_bin" stress schema --type development-challenge > "$tmp_dir/challenge.schema.json"
"$tmp_bin" stress schema --type development-challenge-receipt > "$tmp_dir/challenge-receipt.schema.json"
"$tmp_bin" stress verify-development-challenge-receipt \
  --challenge @eval/results/stress-development-challenge-v1.json \
  --receipt @eval/results/stress-development-challenge-receipt-v1.json > "$tmp_dir/challenge-receipt-validation.json"
"$tmp_bin" stress schema --type held-out-campaign-batch-binding > "$tmp_dir/campaign-batch-binding.schema.json"
"$tmp_bin" stress schema --type held-out-preflight-evidence > "$tmp_dir/preflight-evidence.schema.json"
"$tmp_bin" stress schema --type held-out-preflight-custody > "$tmp_dir/preflight-custody.schema.json"
"$tmp_bin" stress schema --type held-out-execution-permit > "$tmp_dir/execution-permit.schema.json"
"$tmp_bin" stress schema --type held-out-execution-reservation > "$tmp_dir/execution-reservation.schema.json"
"$tmp_bin" stress schema --type held-out-live-batch-evidence > "$tmp_dir/live-batch-evidence.schema.json"
"$tmp_bin" stress schema --type held-out-live-replay-verification > "$tmp_dir/live-replay-verification.schema.json"
"$tmp_bin" stress schema --type held-out-execution-ledger > "$tmp_dir/execution-ledger.schema.json"
"$tmp_bin" stress schema --type held-out-run-seal-v2 > "$tmp_dir/run-seal-v2.schema.json"

jq -e '
  .schema_version == "evalwitness.stress-relation-registry.v1" and
  .catalog_version == "evalwitness.controlled-corruption-stress-catalog.v1" and
  (.relations | length) == 15 and
  .core_cases == 280 and
  .sentinel_cases == 3 and
  .digest == "8d66aff0dc83cd4074d6b90dd157ace04df412931e41c9cecee4def5eff11269"
' "$tmp_dir/catalog-first.json" >/dev/null

jq -e '
  .schema_version == "evalwitness.stress-development-case-study.v1" and
  .case_study_id == "first-listed-candidate-order-one-minimal" and
  .status == "mechanism_demonstration" and
  .observation.outcome == "violated" and
  .original_line_units == 32 and
  .final_line_units == 2 and
  .accepted_reductions == 30 and
  .reduction_percent == 93.75 and
  .empirical_units == 0 and
  .provider_calls == 0 and
  .network_required == false and
  .digest == "b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b"
' "$tmp_dir/case-first.json" >/dev/null

jq -e '
  .valid == true and
  .type == "development-case-study" and
  .digest == "b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b" and
  .empirical_units == 0 and
  .provider_calls == 0 and
  .network_required == false
' "$tmp_dir/case-validation.json" >/dev/null

jq -e '
  .schema_version == "evalwitness.stress-development-challenge.v1" and
  .challenge_id == "candidate-order-one-minimal-portable-challenge" and
  .status == "self_contained_mechanism_challenge" and
  (.fixtures | length) == 4 and
  .expected.case_digest == "b4a7adb23505a88ee0ab79c73957bce5fbd15539a0e5e687590719d3ec4ee84b" and
  .empirical_units == 0 and
  .provider_calls == 0 and
  .network_required == false
' "$tmp_dir/challenge-first.json" >/dev/null

jq -e '
  .schema_version == "evalwitness.stress-development-challenge-receipt.v1" and
  .expected_case_digest == .reproduced_case_digest and
  .fixtures_verified == 4 and
  .original_line_units == 32 and
  .final_line_units == 2 and
  .reduction_attempts == 53 and
  .accepted_reductions == 30 and
  .final_rejection_attempts == 2 and
  .repository_required == false and
  .empirical_units == 0 and
  .provider_calls == 0 and
  .network_required == false and
  .guard_count == 7 and
  .guards_passed == 7 and
  ([.guards[] | .expected_guard == .observed_guard and .passed] | all) and
  .valid == true
' "$tmp_dir/challenge-receipt-first.json" >/dev/null

jq -e '
  .valid == true and
  .receipt_digest == "a00077c238ace087ac88b8f9a7133cdafa09f648173aa8680dfb686da263321d" and
  .challenge_digest == "c583a2379af4d148a5140682fd04f513af0c16520e02fca567d487a387da2901" and
  .guard_count == 7 and
  .guards_passed == 7 and
  .empirical_units == 0 and
  .provider_calls == 0 and
  .network_required == false
' "$tmp_dir/challenge-receipt-validation.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-live-batch-evidence.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-live-batch-evidence.v1"
' "$tmp_dir/live-batch-evidence.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-live-replay-verification.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-live-replay-verification.v1"
' "$tmp_dir/live-replay-verification.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-development-case-study.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-development-case-study.v1"
' "$tmp_dir/case.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-development-challenge.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-development-challenge.v1"
' "$tmp_dir/challenge.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-development-challenge-receipt.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-development-challenge-receipt.v1"
' "$tmp_dir/challenge-receipt.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-campaign-batch-binding.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-campaign-batch-binding.v1"
' "$tmp_dir/campaign-batch-binding.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-preflight-evidence.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-preflight-evidence.v1"
' "$tmp_dir/preflight-evidence.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-preflight-custody.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-preflight-custody.v1"
' "$tmp_dir/preflight-custody.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-execution-permit.v2.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-execution-permit.v2"
' "$tmp_dir/execution-permit.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-execution-reservation.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-execution-reservation.v1"
' "$tmp_dir/execution-reservation.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-execution-ledger.v1.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-execution-ledger.v1"
' "$tmp_dir/execution-ledger.schema.json" >/dev/null

jq -e '
  .["$schema"] == "https://json-schema.org/draft/2020-12/schema" and
  .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-run-seal.v2.json" and
  .additionalProperties == false and
  .properties.schema_version.const == "evalwitness.stress-held-out-run-seal.v2"
' "$tmp_dir/run-seal-v2.schema.json" >/dev/null

terminal_root="eval/trajectories/terminal_trajs/forge_gpt54"
swe_root="eval/trajectories/swebench_verified_trajs"
if [ -d "$terminal_root" ] && [ -d "$swe_root" ]; then
  "$tmp_bin" stress held-out-lock --repository-root . > "$tmp_dir/held-out-lock.json"
  for suffix in first second; do
    EVALWITNESS_API_KEY=must-not-be-read \
      DEEPSEEK_API_KEY=must-not-be-read \
      EVALWITNESS_BASE_URL=http://127.0.0.1:1 \
    "$tmp_bin" stress held-out-campaign --repository-root . --format json > "$tmp_dir/campaign-$suffix.json"
    "$tmp_bin" stress held-out-campaign --repository-root . --format markdown > "$tmp_dir/campaign-$suffix.md"
    EVALWITNESS_API_KEY=must-not-be-read \
      DEEPSEEK_API_KEY=must-not-be-read \
      EVALWITNESS_BASE_URL=http://127.0.0.1:1 \
    "$tmp_bin" stress held-out-readiness --repository-root . --format json > "$tmp_dir/readiness-$suffix.json"
    "$tmp_bin" stress held-out-readiness --repository-root . --format markdown > "$tmp_dir/readiness-$suffix.md"
  done
  cmp "$tmp_dir/campaign-first.json" "$tmp_dir/campaign-second.json"
  cmp "$tmp_dir/campaign-first.md" "$tmp_dir/campaign-second.md"
  cmp "$tmp_dir/campaign-first.json" eval/results/stress-held-out-campaign-plan-v1.json
  cmp "$tmp_dir/campaign-first.md" eval/results/stress-held-out-campaign-plan-v1.md
  cmp "$tmp_dir/readiness-first.json" "$tmp_dir/readiness-second.json"
  cmp "$tmp_dir/readiness-first.md" "$tmp_dir/readiness-second.md"
  cmp "$tmp_dir/readiness-first.json" eval/results/stress-held-out-readiness-refusal-v1.json
  cmp "$tmp_dir/readiness-first.md" eval/results/stress-held-out-readiness-refusal-v1.md
  "$tmp_bin" stress validate \
    --type held-out-campaign-plan \
    --repository-root . \
    --document @eval/results/stress-held-out-campaign-plan-v1.json > "$tmp_dir/campaign-validation.json"
  "$tmp_bin" stress schema --type held-out-campaign-plan > "$tmp_dir/campaign.schema.json"
  "$tmp_bin" stress validate \
    --type held-out-run-readiness-refusal \
    --repository-root . \
    --document @eval/results/stress-held-out-readiness-refusal-v1.json > "$tmp_dir/readiness-validation.json"
  "$tmp_bin" stress schema --type held-out-run-readiness-refusal > "$tmp_dir/readiness.schema.json"
  jq -e '
    .schema_version == "evalwitness.stress-held-out-partition-lock.v1" and
    .source_catalog_version == "evalwitness.controlled-corruption-stress-catalog.v1" and
    .next_catalog_version == "evalwitness.controlled-corruption-stress-catalog.v2" and
    .data_role == "test" and
    .test_cases == 57 and
    .test_cells == 1140 and
    .supported_test_cells == 440 and
    .unsupported_test_cells == 700
  ' "$tmp_dir/held-out-lock.json" >/dev/null
  jq -e '
    .schema_version == "evalwitness.stress-held-out-campaign-plan.v1" and
    .status == "topology_locked_live_bindings_absent" and
    .test_cases == 57 and
    .test_cells == 1140 and
    .supported_test_cells == 440 and
    .structural_unsupported_test_cells == 700 and
    .provider_dependent_arms == 3 and
    .live_provider_arms == 2 and
    .sealed_replay_arms == 1 and
    .zero_cost_arms == 7 and
    .provider_dependent_supported_test_cells == 342 and
    .live_provider_supported_test_cells == 228 and
    .sealed_replay_supported_test_cells == 114 and
    .zero_cost_supported_test_cells == 98 and
    .provider_dependent_verification_inputs == 684 and
    .live_provider_verification_inputs == 456 and
    .sealed_replay_verification_inputs == 228 and
    .planned_provider_dependent_side_repetitions == 2052 and
    .planned_live_provider_side_repetitions == 1368 and
    .planned_sealed_replay_side_repetitions == 684 and
    .planned_zero_cost_repetitions == 294 and
    ([.arms[] | select(.execution_class == "live_provider")] | length) == 2 and
    ([.arms[] | select(.execution_class == "sealed_provider_replay")] | length) == 1 and
    ([.arms[] | select(.execution_class == "deterministic_local")] | length) == 7 and
    ([.live_bindings[]] | all(. == false)) and
    .run_authorized == false and
    .execution_permit_issued == false and
    .provider_calls == 0 and
    .empirical_units == 0 and
    .network_required == false and
    .digest == "7458788aa2635a2eb063eb568f8c37fc7548b2babef1f6ab3893726effacde97"
  ' "$tmp_dir/campaign-first.json" >/dev/null
  jq -e '
    .valid == true and
    .type == "held-out-campaign-plan" and
    .digest == "7458788aa2635a2eb063eb568f8c37fc7548b2babef1f6ab3893726effacde97" and
    .provider_calls == 0 and
    .empirical_units == 0 and
    .network_required == false
  ' "$tmp_dir/campaign-validation.json" >/dev/null
  jq -e '
    .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-campaign-plan.v1.json" and
    .additionalProperties == false and
    .properties.schema_version.const == "evalwitness.stress-held-out-campaign-plan.v1"
  ' "$tmp_dir/campaign.schema.json" >/dev/null
  jq -e '
    .schema_version == "evalwitness.stress-held-out-run-readiness-refusal.v1" and
    .status == "not_ready" and
    .run_authorized == false and
    .execution_permit_issued == false and
    .passed_gates == 1 and
    .blocked_gates == 1 and
    .missing_gates == 6 and
    .provider_calls == 0 and
    .empirical_units == 0 and
    .network_required == false and
    .digest == "46c61709efa76ab4577a89b598a96da046a5ed57d22747446bf253d5d6ab1588"
  ' "$tmp_dir/readiness-first.json" >/dev/null
  jq -e '
    .valid == true and
    .type == "held-out-run-readiness-refusal" and
    .provider_calls == 0 and
    .empirical_units == 0 and
    .network_required == false
  ' "$tmp_dir/readiness-validation.json" >/dev/null
  jq -e '
    .["$id"] == "https://evalwitness.dev/schemas/stress-held-out-run-readiness-refusal.v1.json" and
    .additionalProperties == false and
    .properties.schema_version.const == "evalwitness.stress-held-out-run-readiness-refusal.v1"
  ' "$tmp_dir/readiness.schema.json" >/dev/null
  echo "Stress held-out lock reproduced: 57 cases, 1,140 cells, 440 supported, 700 structural unsupported."
  echo "Held-out campaign plan reproduced: 2 live-provider arms, 1 sealed-replay adapter arm, and 7 deterministic controls; all live bindings absent."
  echo "Held-out readiness refusal reproduced: 1 passed, 1 blocked, 6 missing gates; execution_permit_issued=false."
elif [ "${EVALWITNESS_REQUIRE_CORPUS_SOURCES:-false}" = "true" ]; then
  echo "Stress held-out lock requires the fetched Terminal-Bench and SWE-bench trajectory caches." >&2
  exit 1
else
  echo "Stress held-out lock skipped because fetched eval trajectories are absent."
fi

echo "Stress catalog reproduced: 15 relations over 280 core and 3 scarcity cases."
echo "Development case study reproduced: JSON, Markdown, and SVG; 32 lines reduced to a two-line one-minimal witness, provider_calls=0, empirical_units=0."
echo "Self-contained development challenge reproduced outside the repository: four fixture bytes, 53 reduction attempts, seven guards, provider_calls=0."
