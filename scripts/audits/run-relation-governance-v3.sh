#!/usr/bin/env bash
# Provider-free v3 relation governance, sampling, complete protocol, and design gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d /tmp/evalwitness-relation-v3.XXXXXX)
tmp_bin=$(mktemp /tmp/evalwitness-relation-v3-bin.XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

schema_documents=(
  blind-packet-v3 case-material-v3 condition-probe-v3 condition-probe-batch-v3
  formal-human-comparison-v3 mapping-reveal-v3 pair-judgment-v3 plan-v3
  pilot-sample-v3 pilot-readiness-v3 pilot-change-receipt-v3 pilot-inspection-v3
  pilot-launch-dossier-v3 primary-sample-v3 scarcity-sentinel-v3 private-mapping-v3
  relation-resolution-v3 qualification-answer-key-v3 qualification-report-v3 qualification-set-v3
  replay-receipt-v3 review-assignment-v3 review-bundle-v3 reviewer-handbook-v3
  reviewer-kit-v3 reviewer-record-v3 judgment-batch-v3 prereveal-ambiguity-v3
  study-amendment-v3 terminal-ledger-v3 translation-result-v3
)
if [[ ${#schema_documents[@]} -ne 31 ]]; then
  echo "relation v3 schema inventory must contain 30 protocol schemas plus one scarcity sentinel" >&2
  exit 1
fi
for document in "${schema_documents[@]}"; do
  "$tmp_bin" relation schema --type "$document" > "$tmp_dir/$document.schema.json"
done

journal_schema_documents=(
  pilot-inspection-session pilot-inspection-event pilot-inspection-completion
)
if [[ ${#journal_schema_documents[@]} -ne 3 ]]; then
  echo "relation owner-inspection journal schema inventory must contain three documents" >&2
  exit 1
fi
for document in "${journal_schema_documents[@]}"; do
  "$tmp_bin" relation schema --type "$document" > "$tmp_dir/$document.schema.json"
done

"$tmp_bin" relation schema --type owner-inspection-public-attestation > "$tmp_dir/owner-inspection-public-attestation.schema.json"

for suffix in first second; do
  "$tmp_bin" relation plan-v3 \
    --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
    --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
    --release @eval/governance/controlled-corruption-v3-release.json > "$tmp_dir/plan-$suffix.json"
  "$tmp_bin" relation primary-sample-v3 \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
    --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
    --release @eval/governance/controlled-corruption-v3-release.json > "$tmp_dir/primary-$suffix.json"
  "$tmp_bin" relation scarcity-sentinel-v3 \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --primary-sample "@$tmp_dir/primary-$suffix.json" \
    --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
    --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
    --release @eval/governance/controlled-corruption-v3-release.json > "$tmp_dir/sentinel-$suffix.json"
  "$tmp_bin" relation pilot-sample-v3 \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --primary-sample "@$tmp_dir/primary-$suffix.json" \
    --scarcity-sentinel "@$tmp_dir/sentinel-$suffix.json" \
    --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
    --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
    --release @eval/governance/controlled-corruption-v3-release.json > "$tmp_dir/pilot-$suffix.json"
  "$tmp_bin" relation study-amendment-v3 \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --primary-sample "@$tmp_dir/primary-$suffix.json" \
    --scarcity-sentinel "@$tmp_dir/sentinel-$suffix.json" \
    --pilot-sample "@$tmp_dir/pilot-$suffix.json" \
    --issued-at 2026-08-10T02:50:40Z > "$tmp_dir/amendment-$suffix.json"
done

for document in plan primary sentinel pilot amendment; do
  cmp "$tmp_dir/$document-first.json" "$tmp_dir/$document-second.json"
done
cmp "$tmp_dir/plan-first.json" eval/governance/relation-audit-plan-v3.json
cmp "$tmp_dir/primary-first.json" eval/governance/relation-primary-sample-v3.json
cmp "$tmp_dir/sentinel-first.json" eval/governance/relation-scarcity-sentinel-v3.json
cmp "$tmp_dir/pilot-first.json" eval/governance/relation-pilot-sample-v3.json
cmp "$tmp_dir/amendment-first.json" eval/governance/relation-study-amendment-v3.json

python3 - <<'PY'
import hashlib
import json
import math
import pathlib

paths = {
    "plan": pathlib.Path("eval/governance/relation-audit-plan-v3.json"),
    "primary": pathlib.Path("eval/governance/relation-primary-sample-v3.json"),
    "sentinel": pathlib.Path("eval/governance/relation-scarcity-sentinel-v3.json"),
    "pilot": pathlib.Path("eval/governance/relation-pilot-sample-v3.json"),
    "amendment": pathlib.Path("eval/governance/relation-study-amendment-v3.json"),
}
expected = {
    "plan": ("6eac462cae0a5b626561d5cbea274a5c3a72c78b6cb9d50a8952be6ccbb6fa8c", "b8dab1afa0516204267ac48089fdbd48553e7571a5e35b9437006fe84fc1177b"),
    "primary": ("6b721bcf0fb10e47923b46d92f3c14691cbd0dd98949ab0bf1dc016d8e1c1e43", "fd693e8c4bf582964b7f384d755cf78ff453b870538f597a27972efcc9d574e8"),
    "sentinel": ("ec720a56394249a47eb4c0f7ef618471ce14c69ff8c2c13e1e851350c23e71fb", "cc314db56830e75a96bbc1bf28f65b2b4c9d0afed6d35ef71b2b3ded12f4923b"),
    "pilot": ("3f0a70209575316c78b87f6cb9e7641ff1f4a023eaa86c2fe2598ee14b3c94b4", "4cfe697605898c95e5162ce04ebec0906ed5b784ce1e4b93718f65981ac9fea8"),
    "amendment": ("a9924c62cdb6cea4aa4b9310af9a31ca5ee3cb193645cb30fd2b1135aab0ed94", "f3fdba56a2d49c0416d1e30dd771808a0fa82e995060b85ba04dc77ad6798321"),
}
values = {}
for name, path in paths.items():
    values[name] = json.loads(path.read_text(encoding="utf-8"))
    digest, file_hash = expected[name]
    if values[name]["digest"] != digest:
        raise SystemExit(f"v3 relation {name} digest changed")
    if hashlib.sha256(path.read_bytes()).hexdigest() != file_hash:
        raise SystemExit(f"v3 relation {name} bytes changed")

plan, primary, sentinel, pilot, amendment = (values[name] for name in ("plan", "primary", "sentinel", "pilot", "amendment"))
if (plan["primary_sample_size"], plan["pilot_sample_size"], plan["scarcity_sentinel_size"], plan["sentinel_in_primary_estimand"], plan["held_out_sentinel_available"], plan["empirical_status"]) != (28, 7, 3, False, False, "not_run"):
    raise SystemExit("v3 relation plan sample or claim boundary changed")
if primary["family_counts"] != [{"id": family, "count": 4} for family in [
    "candidate_order_reversal", "causally_independent_event_reorder", "falsified_test_evidence",
    "incomplete_tool_output", "irrelevant_verbosity", "neutral_formatting", "untrusted_score_tag_injection",
]]:
    raise SystemExit("v3 primary seven-family balance changed")
if primary["split_counts"] != [{"id": "calibration", "count": 14}, {"id": "test", "count": 14}]:
    raise SystemExit("v3 primary calibration/test balance changed")
if (primary["selected_cases"], primary["unique_task_groups"], primary["unique_lineage_clusters"], primary["trajectory_pair_units"], primary["candidate_order_units"]) != (28, 28, 28, 24, 4):
    raise SystemExit("v3 primary independence or unit count changed")
if (sentinel["selected_cases"], sentinel["test_cases"], sentinel["primary_overlap"], sentinel["exhaustive"], sentinel["held_out_claim_available"]) != (3, 0, 0, True, False):
    raise SystemExit("v3 scarcity-sentinel boundary changed")
if sentinel["split_counts"] != [{"id": "calibration", "count": 1}, {"id": "development", "count": 2}]:
    raise SystemExit("v3 scarcity-sentinel roles changed")
if (pilot["selected_cases"], pilot["unique_task_groups"], pilot["unique_lineage_clusters"], pilot["primary_overlap"], pilot["scarcity_sentinel_overlap"]) != (7, 7, 7, 0, 0):
    raise SystemExit("v3 pilot independence changed")

def identities(document):
    sources, groups, lineages = set(), set(), set()
    for case in document["cases"]:
        sources.update(case["source_ids"])
        groups.add(case["task_group_id"])
        lineages.update(case["lineage_cluster_ids"])
    return sources, groups, lineages

primary_ids, sentinel_ids, pilot_ids = identities(primary), identities(sentinel), identities(pilot)
for left_name, left, right_name, right in [
    ("primary", primary_ids, "sentinel", sentinel_ids),
    ("primary", primary_ids, "pilot", pilot_ids),
    ("sentinel", sentinel_ids, "pilot", pilot_ids),
]:
    if any(a & b for a, b in zip(left, right)):
        raise SystemExit(f"v3 {left_name}/{right_name} source, task, or lineage overlap appeared")

inference = amendment["inference"]
if not math.isclose(inference["zero_contradiction_upper_bound"], 0.10146573557272465, rel_tol=0, abs_tol=1e-15):
    raise SystemExit("v3 28-group exact upper bound changed")
if [row["detection_probability"] for row in inference["detection_scenarios"]] != [0.7621731147446675, 0.9476652366972639, 0.9980657186886166]:
    raise SystemExit("v3 28-group detection diagnostics changed")
if amendment["empirical_status"] != "not_run" or amendment["external_action_status"] != "not_authorized":
    raise SystemExit("v3 relation amendment fabricated empirical or authorization state")
PY

go test ./internal/relation -run 'Test.*RelationV3|TestControlledCorpusReleaseV3|TestPilotInspectionJournal|TestPilotPackageInventory|TestOwnerInspectionPublicAttestation' -count=1

echo "Controlled-relation v3 protocol reproduced: 30 version-closed workflow schemas, one scarcity-sentinel schema, three guided owner-inspection journal schemas, one public owner-inspection attestation schema, 28 independent core cases, 7 disjoint pilot cases, and 3 exhaustive non-held-out scarcity cases."
echo "Provider calls: not run. Human review: not run. Population inference: not estimated."
