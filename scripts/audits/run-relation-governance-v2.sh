#!/usr/bin/env bash
# Provider-free v2 relation plan, balanced sample, isolated pilot, and amendment gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d /tmp/evalwitness-relation-v2.XXXXXX)
tmp_bin=$(mktemp /tmp/evalwitness-relation-v2-bin.XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness
go test ./internal/relation -run 'TestRelation(V2PostInspectionWorkflowIsVersionClosed|V2PrelaunchSchemasAreVersionMatched|SchemaSurfaceHasNoUntestedDocument)$'

terminal_root="eval/trajectories/terminal_trajs/forge_gpt54"
swe_root="eval/trajectories/swebench_verified_trajs"
if [ ! -d "$terminal_root" ] || [ ! -d "$swe_root" ]; then
  if [ "${EVALWITNESS_REQUIRE_CORPUS_SOURCES:-false}" = "true" ]; then
    echo "Relation governance v2 requires the fetched Terminal-Bench and SWE-bench trajectory caches." >&2
    exit 1
  fi
  echo "Relation governance v2 skipped because fetched eval trajectories are absent."
  exit 0
fi

for schema in plan-v2 primary-sample-v2 pilot-sample-v2 study-amendment-v2 replay-receipt-v2 case-material-v2 blind-packet-v2 private-mapping-v2 qualification-set-v2 qualification-answer-key-v2 qualification-report-v2 reviewer-handbook-v2 review-bundle-v2 pilot-readiness-v2 pilot-change-receipt-v2 pilot-inspection-v2 pilot-launch-dossier-v2 reviewer-record-v2 review-assignment-v2 reviewer-kit-v2 pair-judgment-v2 judgment-batch-v2 prereveal-ambiguity-v2 condition-probe-v2 condition-probe-batch-v2 mapping-reveal-v2 translation-result-v2 relation-resolution-v2 formal-human-comparison-v2 terminal-ledger-v2; do
  "$tmp_bin" relation schema --type "$schema" > "$tmp_dir/$schema.schema.json"
done

"$tmp_bin" mutation corpus plan-v2 --plan @eval/governance/controlled-corruption-v2-plan.json > "$tmp_dir/corpus-plan.json"
"$tmp_bin" mutation corpus audit-v2 \
  --root . \
  --plan "@$tmp_dir/corpus-plan.json" \
  --audited-at 2026-08-09 > "$tmp_dir/corpus-audit.json"
"$tmp_bin" mutation corpus freeze-v2 \
  --plan "@$tmp_dir/corpus-plan.json" \
  --audit "@$tmp_dir/corpus-audit.json" > "$tmp_dir/corpus-spec.json"
cmp "$tmp_dir/corpus-spec.json" eval/governance/controlled-corruption-v2.json
"$tmp_bin" mutation corpus build \
  --root . \
  --spec "@$tmp_dir/corpus-spec.json" > "$tmp_dir/release.json"

build_governance() {
  local suffix=$1
  "$tmp_bin" relation plan-v2 \
    --release "@$tmp_dir/release.json" > "$tmp_dir/plan-$suffix.json"
  "$tmp_bin" relation primary-sample \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --release "@$tmp_dir/release.json" > "$tmp_dir/primary-$suffix.json"
  "$tmp_bin" relation pilot-sample \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --primary-sample "@$tmp_dir/primary-$suffix.json" \
    --release "@$tmp_dir/release.json" > "$tmp_dir/pilot-$suffix.json"
  "$tmp_bin" relation study-amendment \
    --plan "@$tmp_dir/plan-$suffix.json" \
    --primary-sample "@$tmp_dir/primary-$suffix.json" \
    --pilot-sample "@$tmp_dir/pilot-$suffix.json" \
    --issued-at 2026-08-09T22:20:14Z > "$tmp_dir/amendment-$suffix.json"
}

build_governance first
build_governance second

for artifact in plan primary pilot amendment; do
  cmp "$tmp_dir/$artifact-first.json" "$tmp_dir/$artifact-second.json"
done
cmp "$tmp_dir/plan-first.json" eval/governance/relation-audit-plan-v2.json
cmp "$tmp_dir/primary-first.json" eval/governance/relation-primary-sample-v2.json
cmp "$tmp_dir/pilot-first.json" eval/governance/relation-pilot-sample-v2.json
cmp "$tmp_dir/amendment-first.json" eval/governance/relation-study-amendment-v2.json

"$tmp_bin" relation validate --type plan --document "@$tmp_dir/plan-first.json" > "$tmp_dir/plan-validation.json"
"$tmp_bin" relation validate --type primary-sample --document "@$tmp_dir/primary-first.json" > "$tmp_dir/primary-validation.json"
"$tmp_bin" relation validate --type pilot-sample --document "@$tmp_dir/pilot-first.json" > "$tmp_dir/pilot-validation.json"
"$tmp_bin" relation validate --type study-amendment --document "@$tmp_dir/amendment-first.json" > "$tmp_dir/amendment-validation.json"

python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])

def load(name):
    return json.loads((root / name).read_text(encoding="utf-8"))

release = load("release.json")
plan = load("plan-first.json")
primary = load("primary-first.json")
pilot = load("pilot-first.json")
amendment = load("amendment-first.json")

expected = {
    "release": "d0485f3484743a3d4ff907b295c0c9be11db21d2231664e5018fa2f047b6bf11",
    "spec": "94989d548973dad7bfc04418781ed4f25df1b81d6ddd1fbeacc581fefaef0979",
    "program": "30e368b56c42e24bb0cbaf30da1ff9d982a45d6499beca7745db95d2a30ac958",
    "audit": "822d8034a4a75faaf337a4abd6e51743e38104c3ffca9ce7f214e751f5d026db",
    "plan": "dcaba51fcf41b8c3eeb6cedd03ec4646ac6cb80f3457d589122ff4bd82fe4bf0",
    "primary": "5734229be49d6e08ad4b63ed24b1e57ec91074ed2fc45bf598314b51782a3076",
    "pilot": "cdaf1a45f81e6f978a566e3d5256fa203409384aa0df07338032ad2ceee14383",
    "amendment": "107aa68e5699fbc0cb8aa4d1924b892e4500a84de7b9b1aeaec7bddfd8368dbc",
}
if release["digest"] != expected["release"]:
    raise SystemExit("v2 relation governance release digest changed")
if plan["digest"] != expected["plan"] or primary["digest"] != expected["primary"] or pilot["digest"] != expected["pilot"] or amendment["digest"] != expected["amendment"]:
    raise SystemExit("v2 relation governance artifact digest changed")
if (plan["source_corpus_digest"], plan["source_corpus_spec_digest"], plan["source_mutation_program_digest"], plan["source_construct_audit_digest"]) != (expected["release"], expected["spec"], expected["program"], expected["audit"]):
    raise SystemExit("v2 relation plan lost a frozen corpus or construct binding")
if primary["plan_digest"] != expected["plan"] or pilot["primary_sample_digest"] != expected["primary"] or amendment["pilot_sample_digest"] != expected["pilot"]:
    raise SystemExit("v2 relation governance chain is broken")

family_counts = {row["id"]: row["count"] for row in primary["family_counts"]}
split_counts = {row["id"]: row["count"] for row in primary["split_counts"]}
if len(family_counts) != 8 or set(family_counts.values()) != {4} or split_counts != {"calibration": 16, "test": 16}:
    raise SystemExit("v2 primary sample is not family- and split-balanced")
if (primary["selected_cases"], primary["unique_task_groups"], primary["unique_lineage_clusters"]) != (32, 32, 24):
    raise SystemExit("v2 primary sample independence diagnostics changed")
if sum(row["count"] for row in primary["source_format_counts"]) != primary["unique_source_ids"]:
    raise SystemExit("v2 primary source-format denominator is incomplete")

sources = {item["id"]: item for item in release["sources"]}
cases = {item["id"]: item for item in release["cases"]}
pilot_case_ids = {item["case_id"] for item in pilot["cases"]}
if len(pilot_case_ids) != 8 or pilot["primary_overlap"] != 0 or pilot["unique_task_groups"] != 8 or pilot["unique_lineage_clusters"] != 8:
    raise SystemExit("v2 development pilot identity or zero-overlap contract changed")
if any(cases[case_id]["split"] != "development" for case_id in pilot_case_ids):
    raise SystemExit("v2 pilot contains a non-development case")
if len({item["family"] for item in pilot["cases"]}) != 8:
    raise SystemExit("v2 pilot does not cover every governed family exactly once")

primary_selection_digest = primary["selection_digest"]
if not primary_selection_digest or len(primary_selection_digest) != 64:
    raise SystemExit("v2 primary selection digest is invalid")

for reference in pilot["cases"]:
    item = cases[reference["case_id"]]
    if item["manifest"]["split_group_id"] != reference["task_group_id"]:
        raise SystemExit("v2 pilot task-group reference differs from the release")
    if sorted(item["source_ids"]) != sorted(reference["source_ids"]):
        raise SystemExit("v2 pilot source reference differs from the release")

if amendment["primary"]["cases"] != 32 or amendment["primary"]["effective_task_groups"] != 32 or amendment["primary"]["primary_labels"] != 64:
    raise SystemExit("v2 amendment workload changed")
if amendment["empirical_status"] != "not_run" or amendment["external_action_status"] != "not_authorized":
    raise SystemExit("v2 amendment fabricated execution or authorization")
if not 0.089 < amendment["inference"]["zero_contradiction_upper_bound"] < 0.090:
    raise SystemExit("v2 amendment zero-contradiction diagnostic changed")

for name, digest in (("plan", expected["plan"]), ("primary", expected["primary"]), ("pilot", expected["pilot"]), ("amendment", expected["amendment"])):
    validation = load(f"{name}-validation.json")
    if validation != {"valid": True, "type": "study-amendment" if name == "amendment" else "primary-sample" if name == "primary" else "pilot-sample" if name == "pilot" else "plan", "digest": digest}:
        raise SystemExit(f"v2 {name} validation changed")
PY

python3 - "$tmp_dir/plan-first.json" "$tmp_dir/tampered-plan.json" <<'PY'
import json
import pathlib
import sys

plan = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
plan["source_construct_audit_digest"] = "0" * 64
pathlib.Path(sys.argv[2]).write_text(json.dumps(plan, separators=(",", ":")), encoding="utf-8")
PY
if "$tmp_bin" relation validate --type plan --document "@$tmp_dir/tampered-plan.json" >/dev/null 2>&1; then
  echo "v2 relation validation accepted a tampered construct-audit binding" >&2
  exit 1
fi

pilot_case_id=$(python3 -c 'import json; print(json.load(open("eval/governance/relation-pilot-sample-v2.json", encoding="utf-8"))["cases"][0]["case_id"])')
"$tmp_bin" relation materialize \
  --root . \
  --plan @eval/governance/relation-audit-plan-v2.json \
  --release "@$tmp_dir/release.json" \
  --case-id "$pilot_case_id" > "$tmp_dir/case-material-v2.json"
"$tmp_bin" relation validate --type case-material --document "@$tmp_dir/case-material-v2.json" > /dev/null
python3 - "$tmp_dir/case-material-v2.json" <<'PY'
import json
import pathlib
import sys

material = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
expected = {
    "schema_version": "evalwitness.relation-case-material.v2",
    "protocol_version": "evalwitness.controlled-relation-review.v2",
    "source_corpus_spec_digest": "94989d548973dad7bfc04418781ed4f25df1b81d6ddd1fbeacc581fefaef0979",
    "source_mutation_program_digest": "30e368b56c42e24bb0cbaf30da1ff9d982a45d6499beca7745db95d2a30ac958",
    "source_construct_audit_digest": "822d8034a4a75faaf337a4abd6e51743e38104c3ffca9ce7f214e751f5d026db",
    "relation_contract_version": "evalwitness.controlled-relation.v2",
    "evidence_boundary_version": "evalwitness.evidence-boundary.v2",
}
if any(material.get(key) != value for key, value in expected.items()) or len(material.get("construct_firewall_digest", "")) != 64:
    raise SystemExit("v2 relation materialization lost a versioned source, contract, boundary, or construct binding")
PY

echo "Relation governance v2 passed: plan=dcaba51fcf41b8c3eeb6cedd03ec4646ac6cb80f3457d589122ff4bd82fe4bf0 primary=5734229be49d6e08ad4b63ed24b1e57ec91074ed2fc45bf598314b51782a3076 pilot=cdaf1a45f81e6f978a566e3d5256fa203409384aa0df07338032ad2ceee14383 amendment=107aa68e5699fbc0cb8aa4d1924b892e4500a84de7b9b1aeaec7bddfd8368dbc schemas=61 v2_schemas=30 primary_cases=32 task_groups=32 lineage_clusters=24 pilot_cases=8 overlap=0 deterministic=true tamper_rejected=true v2_workflow=true v2_materialization=true providers=not_invoked"
