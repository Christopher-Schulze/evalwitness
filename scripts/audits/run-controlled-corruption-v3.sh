#!/usr/bin/env bash
# Provider-free v3 natural-corpus construct audit and exact negative-result gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d /tmp/evalwitness-corruption-v3.XXXXXX)
tmp_bin=$(mktemp /tmp/evalwitness-corruption-v3-bin.XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" mutation schema --type corpus-development-plan > "$tmp_dir/plan.schema.json"
"$tmp_bin" mutation schema --type corpus-development-audit-v3 > "$tmp_dir/audit.schema.json"
"$tmp_bin" mutation schema --type corpus-release-v3 > "$tmp_dir/release.schema.json"
"$tmp_bin" mutation schema --type verification-evidence-assessment > "$tmp_dir/verification-evidence-assessment.schema.json"
"$tmp_bin" mutation schema --type verification-evidence-challenge > "$tmp_dir/verification-evidence-challenge.schema.json"
for suffix in first second; do
  "$tmp_bin" mutation corpus plan-v3 > "$tmp_dir/plan-$suffix.json"
  "$tmp_bin" mutation verification-evidence build-challenge > "$tmp_dir/verification-evidence-$suffix.json"
done
cmp "$tmp_dir/plan-first.json" "$tmp_dir/plan-second.json"
cmp "$tmp_dir/plan-first.json" eval/governance/controlled-corruption-v3-plan.json
cmp "$tmp_dir/verification-evidence-first.json" "$tmp_dir/verification-evidence-second.json"
cmp "$tmp_dir/verification-evidence-first.json" eval/governance/verification-evidence-challenge-v1.json
"$tmp_bin" mutation verification-evidence validate-challenge \
  --challenge @eval/governance/verification-evidence-challenge-v1.json > "$tmp_dir/verification-evidence-validation.json"

python3 - "$tmp_dir/verification-evidence-validation.json" <<'PY'
import hashlib
import json
import pathlib
import sys

challenge_path = pathlib.Path("eval/governance/verification-evidence-challenge-v1.json")
challenge = json.loads(challenge_path.read_text(encoding="utf-8"))
validation = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
expected_digest = "23c4d331afa7df0fb9a5cce6359ee887319a6d71e394983686c599456c529250"
if challenge["digest"] != expected_digest:
    raise SystemExit("verification-evidence challenge digest changed")
if hashlib.sha256(challenge_path.read_bytes()).hexdigest() != "9bb667a7856df1fbf7943e01d4c5dd7cd99dcca5fec915ba04744b7720d32131":
    raise SystemExit("verification-evidence challenge bytes changed")
if validation != {
    "valid": True,
    "digest": expected_digest,
    "cases": 9,
    "eligible": 2,
    "rejected": 7,
}:
    raise SystemExit("verification-evidence challenge validation summary changed")
PY

terminal_root="eval/trajectories/terminal_trajs/forge_gpt54"
swe_root="eval/trajectories/swebench_verified_trajs"
if [ ! -d "$terminal_root" ] || [ ! -d "$swe_root" ]; then
  if [ "${EVALWITNESS_REQUIRE_CORPUS_SOURCES:-false}" = "true" ]; then
    echo "Controlled corruption v3 audit requires the fetched Terminal-Bench and SWE-bench trajectory caches." >&2
    exit 1
  fi
  echo "Controlled corruption v3 natural-corpus audit skipped because fetched eval trajectories are absent."
  exit 0
fi

for suffix in first second; do
  "$tmp_bin" mutation corpus audit-v3 \
    --root . \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --audited-at 2026-08-10 > "$tmp_dir/audit-$suffix.json"
done
cmp "$tmp_dir/audit-first.json" "$tmp_dir/audit-second.json"
cmp "$tmp_dir/audit-first.json" eval/governance/controlled-corruption-v3-natural-audit.json

for suffix in first second; do
  "$tmp_bin" mutation corpus build-v3 \
    --root . \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --audit "@$tmp_dir/audit-$suffix.json" > "$tmp_dir/release-$suffix.json"
done
cmp "$tmp_dir/release-first.json" "$tmp_dir/release-second.json"
cmp "$tmp_dir/release-first.json" eval/governance/controlled-corruption-v3-release.json

"$tmp_bin" mutation corpus validate-v3-audit \
  --plan @eval/governance/controlled-corruption-v3-plan.json \
  --audit @eval/governance/controlled-corruption-v3-natural-audit.json > "$tmp_dir/validation.json"

"$tmp_bin" mutation corpus validate-v3-release \
  --plan @eval/governance/controlled-corruption-v3-plan.json \
  --audit @eval/governance/controlled-corruption-v3-natural-audit.json \
  --release @eval/governance/controlled-corruption-v3-release.json > "$tmp_dir/release-validation.json"

python3 - "$tmp_dir/validation.json" "$tmp_dir/release-validation.json" <<'PY'
import hashlib
import json
import pathlib
import sys

plan_path = pathlib.Path("eval/governance/controlled-corruption-v3-plan.json")
audit_path = pathlib.Path("eval/governance/controlled-corruption-v3-natural-audit.json")
release_path = pathlib.Path("eval/governance/controlled-corruption-v3-release.json")
validation_path = pathlib.Path(sys.argv[1])
release_validation_path = pathlib.Path(sys.argv[2])

plan = json.loads(plan_path.read_text(encoding="utf-8"))
audit = json.loads(audit_path.read_text(encoding="utf-8"))
release = json.loads(release_path.read_text(encoding="utf-8"))
validation = json.loads(validation_path.read_text(encoding="utf-8"))
release_validation = json.loads(release_validation_path.read_text(encoding="utf-8"))

expected = {
    "plan_digest": "5a7f4f55aafabd03ef9c802a9c48cf198a778228b40ade35e8f074df8d060c85",
    "audit_digest": "af0c0fd56fb498586096a8776e0d40794ee93acf5afda67cc000e576bfcef4d2",
    "plan_sha256": "35414f50cef31e2047959b477566de3119acfd43f78724163e760590f64514e6",
    "audit_sha256": "e207ea5ebf3404cf5f89c943d5dba4e645b034c8dc57c7488d643afd34c956d2",
    "release_digest": "9b4999dafe2d37ea04c298b80a7aba0a1769755fdfd650cd01bf3a9cc31a2e42",
    "release_sha256": "37c96112541ef6991e40e6cd8dbaf79d0a02fd84e61fd36f25222be787e5ef42",
}
if plan["digest"] != expected["plan_digest"] or audit["digest"] != expected["audit_digest"]:
    raise SystemExit("v3 plan or natural-corpus audit digest changed")
if hashlib.sha256(plan_path.read_bytes()).hexdigest() != expected["plan_sha256"]:
    raise SystemExit("v3 governed plan bytes changed")
if hashlib.sha256(audit_path.read_bytes()).hexdigest() != expected["audit_sha256"]:
    raise SystemExit("v3 natural-corpus audit bytes changed")
if release["digest"] != expected["release_digest"]:
    raise SystemExit("v3 controlled release digest changed")
if hashlib.sha256(release_path.read_bytes()).hexdigest() != expected["release_sha256"]:
    raise SystemExit("v3 controlled release bytes changed")
if validation != {
    "valid": True,
    "plan_digest": expected["plan_digest"],
    "audit_digest": expected["audit_digest"],
    "total_attempts": 939,
    "applied_attempts": 689,
    "rejected_attempts": 250,
    "selected_cases": 283,
    "quotas_satisfied": False,
}:
    raise SystemExit("v3 natural-corpus validation summary changed")
if release_validation != {
    "valid": True,
    "release_digest": expected["release_digest"],
    "selected_cases": 283,
    "core_cases": 280,
    "sentinel_cases": 3,
}:
    raise SystemExit("v3 controlled release validation summary changed")
policy = release["policy"]
if policy["core_cases"] != 280 or policy["core_cases_per_family"] != 40 or len(policy["inferential_core_families"]) != 7:
    raise SystemExit("v3 seven-family inferential core changed")
if policy["scarcity_sentinel_family"] != "omitted_test_evidence" or policy["scarcity_sentinel_cases"] != 3:
    raise SystemExit("v3 omitted-evidence scarcity sentinel changed")
if policy["scarcity_sentinel_split_counts"] != [{"id": "calibration", "count": 1}, {"id": "development", "count": 2}]:
    raise SystemExit("v3 omitted-evidence scarcity splits changed")
if policy["sentinel_in_primary_estimand"] or policy["held_out_sentinel_claim_available"] or policy["balanced_eight_family_available"]:
    raise SystemExit("v3 release overclaims the scarcity sentinel")
if audit["quota_shortfalls"] != [{"id": "omitted_test_evidence", "count": 37}]:
    raise SystemExit("v3 natural-corpus scarcity result changed")
if len(audit["coverage"]) != 16:
    raise SystemExit("v3 natural-corpus coverage matrix is incomplete")
coverage = {(row["family"], row["source_format"]): row for row in audit["coverage"]}
swe = coverage[("omitted_test_evidence", "swe_bench_cache_item_json")]
terminal = coverage[("omitted_test_evidence", "terminal_bench_trajectory_json")]
if (swe["attempted"], swe["applied"], swe["rejected"], swe["selected_cases"]) != (80, 0, 80, 0):
    raise SystemExit("v3 SWE omitted-evidence denominator changed")
if (terminal["attempted"], terminal["applied"], terminal["rejected"], terminal["selected_cases"]) != (118, 3, 115, 3):
    raise SystemExit("v3 Terminal omitted-evidence denominator changed")
for row in (swe, terminal):
    if row["rejection_reason_counts"] != [{"id": "unverified_evidence_role", "count": row["rejected"]}]:
        raise SystemExit("v3 omitted-evidence rejection reasons changed")
if audit["findings"] != [
    "complete deterministic attempt universe retained: attempts=939 rejections=250",
    "construct predicates were not relaxed to fill family quotas",
    "family omitted_test_evidence quota shortfall=37",
    "typed invocation and presentation proofs were evaluated on the frozen natural trajectory corpus",
]:
    raise SystemExit("v3 natural-corpus findings changed")
PY

echo "Controlled corruption v3 release reproduced: 939 attempts, 689 applied, 250 rejected, 280 core cases plus 3 omitted-evidence scarcity cases."
echo "Verification-evidence challenge reproduced: 9 cases, 2 eligible, 7 rejected."
echo "Provider calls: not run. Human review: not run. Population inference: not estimated."
