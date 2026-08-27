#!/usr/bin/env bash
# Provider-free v2 development-plan, complete construct-audit, freeze, and release gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d /tmp/evalwitness-corruption-v2.XXXXXX)
tmp_bin=$(mktemp /tmp/evalwitness-corruption-v2-bin.XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

for schema in construct-firewall-v2 construct-firewall-challenge; do
  "$tmp_bin" mutation schema --type "$schema" > "$tmp_dir/$schema.schema.json"
done
for suffix in first second; do
  "$tmp_bin" mutation construct-challenge build > "$tmp_dir/construct-challenge-$suffix.json"
done
cmp "$tmp_dir/construct-challenge-first.json" "$tmp_dir/construct-challenge-second.json"
cmp "$tmp_dir/construct-challenge-first.json" eval/governance/construct-firewall-challenge-v1.json
"$tmp_bin" mutation construct-challenge validate \
  --evidence @eval/governance/construct-firewall-challenge-v1.json > "$tmp_dir/construct-challenge-validation.json"

python3 - "$tmp_dir/construct-challenge-first.json" "$tmp_dir/construct-challenge-validation.json" <<'PY'
import json
import pathlib
import sys

with pathlib.Path(sys.argv[1]).open(encoding="utf-8") as handle:
    challenge = json.load(handle)
with pathlib.Path(sys.argv[2]).open(encoding="utf-8") as handle:
    validation = json.load(handle)

expected_digest = "fe8419e83a9f9bbb1deba048d11da267c87259723f06e1a7fd96f3d971a9dc75"
expected_summary = {
    "cases": 14,
    "v2_applied": 11,
    "v2_rejected": 3,
    "v3_applied": 6,
    "v3_rejected": 8,
    "v2_false_acceptances": 5,
    "v3_repaired_negatives": 5,
    "positive_controls": 6,
    "shared_guards": 3,
}
if challenge["digest"] != expected_digest or challenge["summary"] != expected_summary:
    raise SystemExit("construct-firewall challenge digest or denominators changed")
if challenge["programs"] != {
    "legacy_version": "evalwitness.trajectory-mutation.v2",
    "legacy_program_digest": "30e368b56c42e24bb0cbaf30da1ff9d982a45d6499beca7745db95d2a30ac958",
    "legacy_firewall_schema": "evalwitness.construct-firewall.v1",
    "repaired_version": "evalwitness.trajectory-mutation.v3",
    "repaired_program_digest": "f37ec74f8096a23c5fb0e6696d279f2689f98d7e904fcef038464ca51720b3f8",
    "repaired_firewall_schema": "evalwitness.construct-firewall.v2",
    "invocation_parser": "evalwitness.shell-invocation.v1",
    "presentation_classifier": "evalwitness.presentation-content-kind.v1",
}:
    raise SystemExit("construct-firewall challenge program bindings changed")
if validation != {"valid": True, "evidence_digest": expected_digest, "cases": 14}:
    raise SystemExit("construct-firewall challenge independent validation failed")
for case in challenge["cases"]:
    if case["category"] == "v2_false_acceptance" and (case["v2"]["status"], case["v3"]["status"]) != ("applied", "rejected"):
        raise SystemExit(f"construct-firewall repair changed for {case['id']}")
    if case["category"] == "positive_control" and (case["v2"]["status"], case["v3"]["status"]) != ("applied", "applied"):
        raise SystemExit(f"construct-firewall positive control changed for {case['id']}")
    if case["category"] == "shared_guard" and (case["v2"]["status"], case["v3"]["status"]) != ("rejected", "rejected"):
        raise SystemExit(f"construct-firewall shared guard changed for {case['id']}")
if challenge["claim_boundary"]["provider_calls"] != "not_run" or challenge["claim_boundary"]["human_review"] != "not_run" or challenge["claim_boundary"]["natural_corpus_audit"] != "not_run" or challenge["claim_boundary"]["population_inference"] != "not_estimated":
    raise SystemExit("construct-firewall challenge scientific boundary changed")
PY

terminal_root="eval/trajectories/terminal_trajs/forge_gpt54"
swe_root="eval/trajectories/swebench_verified_trajs"
if [ ! -d "$terminal_root" ] || [ ! -d "$swe_root" ]; then
  if [ "${EVALWITNESS_REQUIRE_CORPUS_SOURCES:-false}" = "true" ]; then
    echo "Controlled corruption v2 audit requires the fetched Terminal-Bench and SWE-bench trajectory caches." >&2
    exit 1
  fi
  echo "Controlled corruption v2 complete construct audit skipped because fetched eval trajectories are absent."
  exit 0
fi

for schema in manifest witness blind-review-packet construct-firewall construct-firewall-v2 construct-repair-evidence construct-firewall-challenge corpus-spec corpus-development-plan corpus-development-audit corpus-release reduction-witness formal-control; do
  "$tmp_bin" mutation schema --type "$schema" > "$tmp_dir/$schema.schema.json"
done

"$tmp_bin" mutation corpus plan-v2 > "$tmp_dir/plan.json"
"$tmp_bin" mutation corpus plan-v2 --plan @eval/governance/controlled-corruption-v2-plan.json > "$tmp_dir/governed-plan.json"
cmp "$tmp_dir/plan.json" "$tmp_dir/governed-plan.json"

run_pipeline() {
  local suffix=$1
  "$tmp_bin" mutation corpus audit-v2 \
    --root . \
    --plan "@$tmp_dir/plan.json" \
    --audited-at 2026-08-09 > "$tmp_dir/audit-$suffix.json"
  "$tmp_bin" mutation corpus freeze-v2 \
    --plan "@$tmp_dir/plan.json" \
    --audit "@$tmp_dir/audit-$suffix.json" > "$tmp_dir/spec-$suffix.json"
  "$tmp_bin" mutation corpus build \
    --root . \
    --spec "@$tmp_dir/spec-$suffix.json" > "$tmp_dir/release-$suffix.json"
  "$tmp_bin" mutation corpus validate \
    --release "@$tmp_dir/release-$suffix.json" > "$tmp_dir/validation-$suffix.json"
  "$tmp_bin" mutation corpus verify-v2-audit \
    --plan "@$tmp_dir/plan.json" \
    --audit "@$tmp_dir/audit-$suffix.json" \
    --release "@$tmp_dir/release-$suffix.json" > "$tmp_dir/audit-verification-$suffix.json"
}

run_pipeline first
run_pipeline second

for suffix in first second; do
  "$tmp_bin" mutation construct-repair build \
    --plan "@$tmp_dir/plan.json" \
    --audit "@$tmp_dir/audit-$suffix.json" \
    --release "@$tmp_dir/release-$suffix.json" > "$tmp_dir/construct-repair-$suffix.json"
done

cmp "$tmp_dir/audit-first.json" "$tmp_dir/audit-second.json"
cmp "$tmp_dir/spec-first.json" "$tmp_dir/spec-second.json"
cmp "$tmp_dir/spec-first.json" eval/governance/controlled-corruption-v2.json
cmp "$tmp_dir/release-first.json" "$tmp_dir/release-second.json"
cmp "$tmp_dir/construct-repair-first.json" "$tmp_dir/construct-repair-second.json"
cmp "$tmp_dir/construct-repair-first.json" eval/governance/construct-repair-evidence-v1.json

"$tmp_bin" mutation construct-repair validate \
  --evidence @eval/governance/construct-repair-evidence-v1.json \
  --plan "@$tmp_dir/plan.json" \
  --audit "@$tmp_dir/audit-first.json" \
  --release "@$tmp_dir/release-first.json" > "$tmp_dir/construct-repair-validation.json"

python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])

def load(name):
    with (root / name).open(encoding="utf-8") as handle:
        return json.load(handle)

plan = load("plan.json")
audit = load("audit-first.json")
spec = load("spec-first.json")
release = load("release-first.json")
validation = load("validation-first.json")
verification = load("audit-verification-first.json")
construct_repair = load("construct-repair-first.json")
construct_repair_validation = load("construct-repair-validation.json")

expected = {
    "plan": "75ce0c5eb2ab464d48685b1342af5ed64c194e03a671723eaaf772514fa87873",
    "audit": "822d8034a4a75faaf337a4abd6e51743e38104c3ffca9ce7f214e751f5d026db",
    "spec": "94989d548973dad7bfc04418781ed4f25df1b81d6ddd1fbeacc581fefaef0979",
    "release": "d0485f3484743a3d4ff907b295c0c9be11db21d2231664e5018fa2f047b6bf11",
    "construct_repair": "b166f124cc7e3f31b26676bba08602fca2d29a9df95f0f80a91c421f34e137c9",
}
if plan["digest"] != expected["plan"]:
    raise SystemExit("v2 development-plan digest changed")
if audit["digest"] != expected["audit"] or audit["plan_digest"] != expected["plan"]:
    raise SystemExit("v2 complete construct-audit digest or plan binding changed")
if release["spec_digest"] != expected["spec"] or release["digest"] != expected["release"]:
    raise SystemExit("v2 governed spec or release digest changed")
if spec["development_audit"]["construct_audit_digest"] != expected["audit"]:
    raise SystemExit("v2 governed spec does not bind the complete construct audit")
if audit["quotas_satisfied"] is not True or audit["quota_shortfalls"] != []:
    raise SystemExit("v2 construct audit has a family quota shortfall")
if (audit["source_tasks"], len(audit["sources"]), audit["total_attempts"], audit["applied_attempts"], audit["rejected_attempts"], audit["selected_cases"]) != (100, 200, 873, 738, 135, 320):
    raise SystemExit("v2 complete construct-audit denominators changed")
if len(release["cases"]) != 320 or len(release["construct_rejections"]) != 135:
    raise SystemExit("v2 release selection or rejection retention changed")
if any(row["count"] != 40 for row in release["mutation_family_counts"]):
    raise SystemExit("v2 release family balance changed")
coverage = {(row["family"], row["source_format"]): row for row in audit["coverage"]}
if len(coverage) != 16:
    raise SystemExit("v2 audit does not report every family and source-format denominator")
families = set(plan["primary_families"])
formats = {"swe_bench_cache_item_json", "terminal_bench_trajectory_json"}
if set(coverage) != {(family, source_format) for family in families for source_format in formats}:
    raise SystemExit("v2 audit coverage matrix is incomplete")
for family in families:
    selected = sum(coverage[(family, source_format)]["selected_cases"] for source_format in formats)
    if selected != 40:
        raise SystemExit(f"v2 audit family {family} selected {selected}/40 cases")
terminal_omission = coverage[("omitted_test_evidence", "terminal_bench_trajectory_json")]
reason_counts = {row["id"]: row["count"] for row in terminal_omission["rejection_reason_counts"]}
if terminal_omission["rejected"] != 96 or reason_counts != {"unverified_evidence_role": 96}:
    raise SystemExit("v2 audit evidence-lineage scarcity result changed")
if validation.get("valid") is not True or validation.get("digest") != expected["release"]:
    raise SystemExit("v2 release validation failed")
if verification != {"valid": True, "release_digest": expected["release"], "audit_digest": expected["audit"]}:
    raise SystemExit("v2 release-to-audit verification failed")
if construct_repair["corpus"] != {
    "audited_at": "2026-08-09",
    "plan_digest": expected["plan"],
    "audit_digest": expected["audit"],
    "release_digest": expected["release"],
    "mutation_program_digest": "30e368b56c42e24bb0cbaf30da1ff9d982a45d6499beca7745db95d2a30ac958",
    "sources": 200,
    "source_tasks": 100,
    "total_attempts": 873,
    "applied_attempts": 738,
    "rejected_attempts": 135,
    "selected_cases": 320,
    "coverage_cells": 16,
}:
    raise SystemExit("construct-repair corpus binding changed")
expected_rejections = {
    "generic_completion_evidence_role": "unverified_evidence_role",
    "pathological_executable_text_presentation": "unnatural_formatting",
    "shared_tool_transaction_reorder": "transaction_dependency",
}
if construct_repair["summary"] != {"fixtures": 3, "legacy_accepted": 3, "corrected_rejected": 3}:
    raise SystemExit("construct-repair summary changed")
if construct_repair["digest"] != expected["construct_repair"]:
    raise SystemExit("construct-repair evidence digest changed")
for case in construct_repair["cases"]:
    if case["legacy_manifest"]["program"]["version"] != "evalwitness.trajectory-mutation.v1" or case["legacy_manifest"]["witness"]["label_state"] != "proven":
        raise SystemExit(f"construct-repair legacy acceptance changed for {case['id']}")
    if case["corrected_firewall"]["program_version"] != "evalwitness.trajectory-mutation.v2" or case["corrected_firewall"]["status"] != "rejected" or case["corrected_firewall"]["rejection_reasons"] != [expected_rejections[case["id"]]]:
        raise SystemExit(f"construct-repair corrected rejection changed for {case['id']}")
if construct_repair["claim_boundary"] != {
    "evidence_kind": "deterministic_public_regression_fixtures",
    "provider_calls": "not_run",
    "human_review": "not_run",
    "population_inference": "not_estimated",
    "supported_claim": "v1 accepts each frozen synthetic regression fixture while v2 rejects it under the recorded closed reason",
    "unsupported_claims": ["human construct validity", "population prevalence of the defects", "provider or verifier performance"],
}:
    raise SystemExit("construct-repair claim boundary changed")
if construct_repair_validation != {
    "valid": True,
    "evidence_digest": construct_repair["digest"],
    "plan_digest": expected["plan"],
    "audit_digest": expected["audit"],
    "release_digest": expected["release"],
    "cases": 3,
}:
    raise SystemExit("construct-repair independent validation failed")
PY

python3 - "$tmp_dir/audit-first.json" "$tmp_dir/tampered-audit.json" <<'PY'
import json
import pathlib
import sys

source = pathlib.Path(sys.argv[1])
target = pathlib.Path(sys.argv[2])
with source.open(encoding="utf-8") as handle:
    audit = json.load(handle)
audit["selected_case_ids"][0] = "mutation-" + "0" * 64
with target.open("w", encoding="utf-8") as handle:
    json.dump(audit, handle, separators=(",", ":"), ensure_ascii=False)
PY
if "$tmp_bin" mutation corpus freeze-v2 --plan "@$tmp_dir/plan.json" --audit "@$tmp_dir/tampered-audit.json" >/dev/null 2>&1; then
  echo "v2 corpus freeze accepted a tampered construct audit" >&2
  exit 1
fi

echo "Controlled corruption v2 passed: plan=75ce0c5eb2ab464d48685b1342af5ed64c194e03a671723eaaf772514fa87873 audit=822d8034a4a75faaf337a4abd6e51743e38104c3ffca9ce7f214e751f5d026db release=d0485f3484743a3d4ff907b295c0c9be11db21d2231664e5018fa2f047b6bf11 construct_repair=b166f124cc7e3f31b26676bba08602fca2d29a9df95f0f80a91c421f34e137c9 sources=200 tasks=100 attempts=873 applied=738 rejected=135 cases=320 families=8 coverage_cells=16 construct_repair_cases=3 deterministic=true tamper_rejected=true providers=not_invoked"
