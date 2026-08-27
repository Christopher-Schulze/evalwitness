#!/usr/bin/env bash
# Provider-free controlled-relation objective, sample-binding, and isolation gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d /tmp/evalwitness-relation-construct-XXXXXXXX)
tmp_bin=$(mktemp /tmp/evalwitness-relation-construct-bin-XXXXXXXX)
trap 'rm -rf "$tmp_dir"; rm -f "$tmp_bin"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

relation_schemas=(blind-packet case-material condition-probe condition-probe-batch formal-human-comparison judgment-batch mapping-reveal normalized-observations pair-judgment pilot-change-receipt pilot-inspection pilot-launch-dossier plan pilot-readiness pilot-sample prereveal-ambiguity primary-sample private-mapping qualification-answer-key qualification-report qualification-set relation-resolution replay-receipt review-assignment review-bundle reviewer-handbook reviewer-kit reviewer-record study-amendment terminal-ledger translation-result)
for schema in "${relation_schemas[@]}"; do
  "$tmp_bin" relation schema --type "$schema" > "$tmp_dir/$schema.schema.json"
done
schema_count=${#relation_schemas[@]}
"$tmp_bin" relation plan --plan @eval/governance/relation-audit-plan-v1.json > "$tmp_dir/plan.json"
"$tmp_bin" relation validate --type plan --document @eval/governance/relation-audit-plan-v1.json > "$tmp_dir/plan-validation.json"
python3 - "$tmp_dir/outcome-packet.json" <<'PY'
import json
import pathlib
import sys

qualification = json.loads(pathlib.Path("eval/governance/outcome-qualification-v1.json").read_text(encoding="utf-8"))
pathlib.Path(sys.argv[1]).write_text(json.dumps(qualification["cases"][0]["packet"], separators=(",", ":")), encoding="utf-8")
PY
if "$tmp_bin" relation validate --type blind-packet --document @"$tmp_dir/outcome-packet.json" > /dev/null 2>&1; then
  echo "relation validation accepted a single-trajectory outcome packet" >&2
  exit 1
fi
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
fixtures = {
    "support-observations.json": [
        {"axis": "evidence_strength", "rating": "original"},
        {"axis": "information_sufficiency", "rating": "sufficient"},
        {"axis": "semantic_task_quality", "rating": "equal"},
    ],
    "contradict-observations.json": [
        {"axis": "evidence_strength", "rating": "transformed"},
        {"axis": "information_sufficiency", "rating": "sufficient"},
        {"axis": "semantic_task_quality", "rating": "equal"},
    ],
    "unresolved-observations.json": [
        {"axis": "evidence_strength", "rating": "original"},
        {"axis": "information_sufficiency", "rating": "insufficient"},
        {"axis": "semantic_task_quality", "rating": "equal"},
    ],
    "outcome-shaped-observations.json": [
        {"axis": "evidence_strength", "rating": "solved"},
        {"axis": "information_sufficiency", "rating": "sufficient"},
        {"axis": "semantic_task_quality", "rating": "equal"},
    ],
}
for name, value in fixtures.items():
    (root / name).write_text(json.dumps(value, separators=(",", ":")), encoding="utf-8")
PY
for state in support contradict unresolved; do
  "$tmp_bin" relation translate \
    --plan @eval/governance/relation-audit-plan-v1.json \
    --family omitted_test_evidence \
    --observations @"$tmp_dir/$state-observations.json" > "$tmp_dir/$state-translation.json"
  "$tmp_bin" relation validate --type translation-result --document @"$tmp_dir/$state-translation.json" > /dev/null
done
if "$tmp_bin" relation translate \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --family omitted_test_evidence \
  --observations @"$tmp_dir/outcome-shaped-observations.json" > /dev/null 2>&1; then
  echo "relation translation accepted an outcome-style rating" >&2
  exit 1
fi

if "$tmp_bin" relation validate --type plan --document @eval/governance/outcome-adjudication-v1.json > /dev/null 2>&1; then
  echo "relation validation accepted an outcome-adjudication plan" >&2
  exit 1
fi
if "$tmp_bin" relation validate --type primary-sample --document @eval/governance/outcome-mutation-sample-v1.json > /dev/null 2>&1; then
  echo "relation validation accepted an outcome sample commitment" >&2
  exit 1
fi
if "$tmp_bin" outcome validate --type sample-commitment --document @eval/governance/relation-primary-sample-v1.json > /dev/null 2>&1; then
  echo "outcome validation accepted a controlled-relation sample" >&2
  exit 1
fi

terminal_root="eval/trajectories/terminal_trajs/forge_gpt54"
swe_root="eval/trajectories/swebench_verified_trajs"
if [[ ! -d "$terminal_root" || ! -d "$swe_root" ]]; then
  echo "Relation construct core passed; exact 31-case binding skipped because fetched eval trajectories are absent."
  exit 0
fi

"$tmp_bin" mutation corpus build --root . --spec @eval/governance/controlled-corruption-v1.json > "$tmp_dir/release.json"
"$tmp_bin" relation primary-sample \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --release @"$tmp_dir/release.json" > "$tmp_dir/primary-sample.json"
"$tmp_bin" relation validate --type primary-sample --document @"$tmp_dir/primary-sample.json" > "$tmp_dir/sample-validation.json"
"$tmp_bin" relation pilot-sample \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --primary-sample @"$tmp_dir/primary-sample.json" \
  --release @"$tmp_dir/release.json" > "$tmp_dir/pilot-sample.json"
"$tmp_bin" relation validate --type pilot-sample --document @"$tmp_dir/pilot-sample.json" > "$tmp_dir/pilot-validation.json"
python3 - "$tmp_dir/pilot-sample.json" "$tmp_dir/pair-case-id.txt" "$tmp_dir/trajectory-case-id.txt" <<'PY'
import json
import pathlib
import sys

pilot = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
pair = next(case for case in pilot["cases"] if case["unit"] == "candidate_pair_orderings")
trajectory = next(case for case in pilot["cases"] if case["unit"] == "trajectory_pair")
pathlib.Path(sys.argv[2]).write_text(pair["case_id"] + "\n", encoding="utf-8")
pathlib.Path(sys.argv[3]).write_text(trajectory["case_id"] + "\n", encoding="utf-8")
PY
read -r pair_case_id < "$tmp_dir/pair-case-id.txt"
read -r trajectory_case_id < "$tmp_dir/trajectory-case-id.txt"
"$tmp_bin" relation replay --root . --release @"$tmp_dir/release.json" --case-id "$pair_case_id" > "$tmp_dir/pair-replay.json"
"$tmp_bin" relation replay --root . --release @"$tmp_dir/release.json" --case-id "$trajectory_case_id" > "$tmp_dir/trajectory-replay.json"
"$tmp_bin" relation validate --type replay-receipt --document @"$tmp_dir/pair-replay.json" > /dev/null
"$tmp_bin" relation validate --type replay-receipt --document @"$tmp_dir/trajectory-replay.json" > /dev/null
"$tmp_bin" relation materialize --root . --plan @eval/governance/relation-audit-plan-v1.json --release @"$tmp_dir/release.json" --case-id "$pair_case_id" > "$tmp_dir/pair-material.json"
"$tmp_bin" relation materialize --root . --plan @eval/governance/relation-audit-plan-v1.json --release @"$tmp_dir/release.json" --case-id "$trajectory_case_id" > "$tmp_dir/trajectory-material.json"
"$tmp_bin" relation validate --type case-material --document @"$tmp_dir/pair-material.json" > /dev/null
"$tmp_bin" relation validate --type case-material --document @"$tmp_dir/trajectory-material.json" > /dev/null
python3 - "$tmp_dir/relation-key.hex" <<'PY'
import os
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
path.write_text("42" * 32 + "\n", encoding="ascii")
os.chmod(path, 0o600)
PY
python3 - "$tmp_dir/relation-package-qualification-key.hex" <<'PY'
import os
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
path.write_text("43" * 32 + "\n", encoding="ascii")
os.chmod(path, 0o600)
PY
private_root="$tmp_dir/private-vault"
"$tmp_bin" relation packet \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --release @"$tmp_dir/release.json" \
  --material @"$tmp_dir/pair-material.json" \
  --key-file "$tmp_dir/relation-key.hex" \
  --key-id relation-audit-fixture-key \
  --private-root "$private_root" > "$tmp_dir/pair-packet.json"
"$tmp_bin" relation packet \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --release @"$tmp_dir/release.json" \
  --material @"$tmp_dir/trajectory-material.json" \
  --key-file "$tmp_dir/relation-key.hex" \
  --key-id relation-audit-fixture-key \
  --private-root "$private_root" > "$tmp_dir/trajectory-packet.json"
"$tmp_bin" relation validate --type blind-packet --document @"$tmp_dir/pair-packet.json" > /dev/null
"$tmp_bin" relation validate --type blind-packet --document @"$tmp_dir/trajectory-packet.json" > /dev/null
if "$tmp_bin" outcome validate --type blind-packet --document @"$tmp_dir/pair-packet.json" > /dev/null 2>&1; then
  echo "outcome validation accepted a controlled-relation packet" >&2
  exit 1
fi
mapping_files=("$private_root"/mappings/*.json)
if [[ ${#mapping_files[@]} -ne 2 ]]; then
  echo "relation packet generation did not publish exactly two private mappings" >&2
  exit 1
fi
for mapping_file in "${mapping_files[@]}"; do
  "$tmp_bin" relation validate --type private-mapping --document @"$mapping_file" > /dev/null
  if [[ $(stat -f '%Lp' "$mapping_file" 2>/dev/null || stat -c '%a' "$mapping_file") != 600 ]]; then
    echo "relation private mapping is not mode 0600" >&2
    exit 1
  fi
done
mapping_hashes_before=$(shasum -a 256 "${mapping_files[@]}")
if "$tmp_bin" relation packet \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --release @"$tmp_dir/release.json" \
  --material @"$tmp_dir/pair-material.json" \
  --key-file "$tmp_dir/relation-key.hex" \
  --key-id relation-audit-fixture-key \
  --private-root "$private_root" > /dev/null 2>&1; then
  echo "relation private mapping publication overwrote an existing content address" >&2
  exit 1
fi
mapping_hashes_after=$(shasum -a 256 "${mapping_files[@]}")
if [[ "$mapping_hashes_before" != "$mapping_hashes_after" ]]; then
  echo "relation exclusive mapping publication changed existing owner-only content" >&2
  exit 1
fi
"$tmp_bin" relation qualification \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --key-file "$tmp_dir/relation-key.hex" --key-id relation-qualification-fixture-key \
  --private-root "$private_root" > "$tmp_dir/qualification-set.json"
qualification_key_files=("$private_root"/qualification-keys/*.json)
if [[ ${#qualification_key_files[@]} -ne 1 ]]; then
  echo "relation qualification did not publish exactly one owner-only answer key" >&2
  exit 1
fi
qualification_key_file=${qualification_key_files[0]}
"$tmp_bin" relation validate --type qualification-set --document @"$tmp_dir/qualification-set.json" > /dev/null
"$tmp_bin" relation validate --type qualification-answer-key --document @"$qualification_key_file" > /dev/null
if [[ $(stat -f '%Lp' "$qualification_key_file" 2>/dev/null || stat -c '%a' "$qualification_key_file") != 600 ]]; then
  echo "relation qualification answer key is not mode 0600" >&2
  exit 1
fi
if "$tmp_bin" relation qualification \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --key-file "$tmp_dir/relation-key.hex" --key-id relation-qualification-fixture-key \
  --private-root "$private_root" > /dev/null 2>&1; then
  echo "relation qualification overwrote an existing answer key" >&2
  exit 1
fi
"$tmp_bin" relation handbook \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --qualification-set @"$tmp_dir/qualification-set.json" > "$tmp_dir/reviewer-handbook.json"
"$tmp_bin" relation validate --type reviewer-handbook --document @"$tmp_dir/reviewer-handbook.json" > /dev/null
readiness_private_root="$tmp_dir/readiness-private-vault"
readiness_index=0
while IFS= read -r readiness_case_id; do
  if ! "$tmp_bin" relation materialize --root . \
    --plan @eval/governance/relation-audit-plan-v1.json --release @"$tmp_dir/release.json" \
    --case-id "$readiness_case_id" > "$tmp_dir/readiness-material-$readiness_index.json"; then
    echo "relation pilot readiness failed to materialize case $readiness_case_id" >&2
    exit 1
  fi
  "$tmp_bin" relation packet \
    --plan @eval/governance/relation-audit-plan-v1.json --release @"$tmp_dir/release.json" \
    --material @"$tmp_dir/readiness-material-$readiness_index.json" \
    --key-file "$tmp_dir/relation-key.hex" --key-id relation-readiness-fixture-key \
    --private-root "$readiness_private_root" > "$tmp_dir/readiness-packet-$readiness_index.json"
  readiness_index=$((readiness_index + 1))
done < <(python3 -c 'import json; print("\n".join(case["case_id"] for case in json.load(open("eval/governance/relation-pilot-sample-v1.json", encoding="utf-8"))["cases"]))')
if [[ "$readiness_index" -ne 8 ]]; then
  echo "relation pilot readiness did not materialize all eight cases" >&2
  exit 1
fi
python3 - "$tmp_dir" <<'PY'
import json
import os
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
packets = [json.loads((root / f"readiness-packet-{index}.json").read_text(encoding="utf-8")) for index in range(8)]
mappings = [json.loads(path.read_text(encoding="utf-8")) for path in sorted((root / "readiness-private-vault" / "mappings").glob("*.json"))]
if len(mappings) != 8:
    raise SystemExit("relation pilot readiness did not publish eight private mappings")
(root / "readiness-packets.json").write_text(json.dumps(packets, separators=(",", ":")), encoding="utf-8")
(root / "readiness-mappings.json").write_text(json.dumps(mappings, separators=(",", ":")), encoding="utf-8")
os.chmod(root / "readiness-mappings.json", 0o600)
PY
"$tmp_bin" relation bundle \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --sample-digest "$(python3 -c 'import json; print(json.load(open("eval/governance/relation-pilot-sample-v1.json"))["digest"])')" \
  --data-role development_pilot --packets @"$tmp_dir/readiness-packets.json" --mappings @"$tmp_dir/readiness-mappings.json" \
  --qualification-set @"$tmp_dir/qualification-set.json" --handbook @"$tmp_dir/reviewer-handbook.json" \
  --created-at 2026-08-09T17:50:00Z > "$tmp_dir/readiness-bundle.json"
"$tmp_bin" relation pilot-readiness \
  --plan @eval/governance/relation-audit-plan-v1.json --pilot-sample @eval/governance/relation-pilot-sample-v1.json \
  --bundle @"$tmp_dir/readiness-bundle.json" --mappings @"$tmp_dir/readiness-mappings.json" \
  --qualification-set @"$tmp_dir/qualification-set.json" --handbook @"$tmp_dir/reviewer-handbook.json" \
  --prepared-at 2026-08-09T17:51:00Z > "$tmp_dir/pilot-readiness.json"
"$tmp_bin" relation validate --type pilot-readiness --document @"$tmp_dir/pilot-readiness.json" > /dev/null
"$tmp_bin" relation render-pilot-inspection \
  --readiness @"$tmp_dir/pilot-readiness.json" --bundle @"$tmp_dir/readiness-bundle.json" \
  --mappings @"$tmp_dir/readiness-mappings.json" --handbook @"$tmp_dir/reviewer-handbook.json" \
  > "$tmp_dir/pilot-inspection-workbook.md"
python3 - "$tmp_dir" <<'PY'
import json
import os
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
readiness = json.loads((root / "pilot-readiness.json").read_text(encoding="utf-8"))
decisions = []
for check in readiness["packet_checks"]:
    decisions.append({
        "packet_id": check["packet_id"],
        "task_context": "passed",
        "evidence_alignment": "passed",
        "transformation_isolation": "passed",
        "information_sufficiency": "passed",
        "blinding_integrity": "passed",
        "rubric_applicability": "passed",
        "redistribution_boundary": "passed",
        "candidate_order": "passed" if check["unit"] == "candidate_pair_orderings" else "not_applicable",
        "reason_codes": [],
    })
(root / "pilot-inspection-decisions.json").write_text(json.dumps(decisions, separators=(",", ":")), encoding="utf-8")
invalid = json.loads(json.dumps(decisions))
invalid[0]["task_context"] = "failed"
(root / "invalid-pilot-inspection-decisions.json").write_text(json.dumps(invalid, separators=(",", ":")), encoding="utf-8")
readiness_mappings = json.loads((root / "readiness-mappings.json").read_text(encoding="utf-8"))
(root / "incomplete-readiness-mappings.json").write_text(json.dumps(readiness_mappings[1:], separators=(",", ":")), encoding="utf-8")
os.chmod(root / "pilot-inspection-decisions.json", 0o600)
os.chmod(root / "invalid-pilot-inspection-decisions.json", 0o600)
os.chmod(root / "incomplete-readiness-mappings.json", 0o600)
PY
"$tmp_bin" relation pilot-inspection \
  --readiness @"$tmp_dir/pilot-readiness.json" --bundle @"$tmp_dir/readiness-bundle.json" \
  --mappings @"$tmp_dir/readiness-mappings.json" --decisions @"$tmp_dir/pilot-inspection-decisions.json" \
  --inspector-alias synthetic-owner-audit-fixture --inspected-at 2026-08-09T17:52:00Z \
  --private-root "$tmp_dir/pilot-inspection-vault" > "$tmp_dir/pilot-inspection-receipt.json"
pilot_inspection_digest="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["digest"])' "$tmp_dir/pilot-inspection-receipt.json")"
pilot_inspection_file="$tmp_dir/pilot-inspection-vault/pilot-inspections/$pilot_inspection_digest.json"
"$tmp_bin" relation validate --type pilot-inspection --document @"$pilot_inspection_file" > /dev/null
"$tmp_bin" relation verify-pilot-inspection \
  --inspection @"$pilot_inspection_file" --readiness @"$tmp_dir/pilot-readiness.json" \
  --bundle @"$tmp_dir/readiness-bundle.json" --mappings @"$tmp_dir/readiness-mappings.json" \
  > "$tmp_dir/pilot-inspection-verification.json"
if "$tmp_bin" relation verify-pilot-inspection \
  --inspection @"$pilot_inspection_file" --readiness @"$tmp_dir/pilot-readiness.json" \
  --bundle @"$tmp_dir/readiness-bundle.json" --mappings @"$tmp_dir/incomplete-readiness-mappings.json" \
  > /dev/null 2>&1; then
  echo "relation pilot inspection verification accepted incomplete private mappings" >&2
  exit 1
fi
if "$tmp_bin" relation pilot-inspection \
  --readiness @"$tmp_dir/pilot-readiness.json" --bundle @"$tmp_dir/readiness-bundle.json" \
  --mappings @"$tmp_dir/readiness-mappings.json" --decisions @"$tmp_dir/invalid-pilot-inspection-decisions.json" \
  --inspector-alias synthetic-owner-audit-fixture --inspected-at 2026-08-09T17:52:00Z > /dev/null 2>&1; then
  echo "relation pilot inspection accepted a failed dimension without its canonical reason" >&2
  exit 1
fi
relation_pilot_package="$tmp_dir/relation-pilot-package"
scripts/build/prepare-relation-pilot.sh \
  --binary "$tmp_bin" \
  --packet-key-file "$tmp_dir/relation-key.hex" --packet-key-id relation-pilot-package-packet-key \
  --qualification-key-file "$tmp_dir/relation-package-qualification-key.hex" --qualification-key-id relation-pilot-package-qualification-key \
  --package-root "$relation_pilot_package" \
  --bundle-created-at 2026-08-09T19:00:00Z \
  --readiness-prepared-at 2026-08-09T19:01:00Z \
  --dossier-prepared-at 2026-08-09T19:02:00Z > "$tmp_dir/relation-pilot-package-prepare.log"
scripts/audits/verify-relation-pilot-package.sh \
  --binary "$tmp_bin" --package-root "$relation_pilot_package" \
  > "$tmp_dir/relation-pilot-package-verification.log"
python3 - "$relation_pilot_package/pilot-launch-dossier.json" "$tmp_dir/tampered-pilot-launch-dossier.json" <<'PY'
import json
import os
import pathlib
import sys

source = pathlib.Path(sys.argv[1])
target = pathlib.Path(sys.argv[2])
dossier = json.loads(source.read_text(encoding="utf-8"))
dossier["external_actions"][0]["status"] = "authorized"
target.write_text(json.dumps(dossier, separators=(",", ":")), encoding="utf-8")
os.chmod(target, 0o600)
PY
if "$tmp_bin" relation validate --type pilot-launch-dossier \
  --document @"$tmp_dir/tampered-pilot-launch-dossier.json" > /dev/null 2>&1; then
  echo "relation pilot launch dossier accepted an authorized external action" >&2
  exit 1
fi
if "$tmp_bin" relation pilot-launch-dossier \
  --plan @"$relation_pilot_package/relation-audit-plan.json" \
  --pilot-sample @"$relation_pilot_package/relation-pilot-sample.json" \
  --bundle @"$relation_pilot_package/review-bundle.json" \
  --mappings @"$tmp_dir/incomplete-readiness-mappings.json" \
  --qualification-set @"$relation_pilot_package/qualification-set.json" \
  --handbook @"$relation_pilot_package/reviewer-handbook.json" \
  --readiness @"$relation_pilot_package/pilot-readiness.json" \
  --prepared-at 2026-08-09T19:02:00Z > /dev/null 2>&1; then
  echo "relation pilot launch dossier accepted incomplete or mismatched private custody" >&2
  exit 1
fi
python3 - "$relation_pilot_package" "$tmp_dir/relation-pilot-package-before.json" <<'PY'
import hashlib
import json
import pathlib
import stat
import sys

root = pathlib.Path(sys.argv[1])
entries = []
for path in [root, *sorted(root.rglob("*"))]:
    entries.append({
        "path": "." if path == root else str(path.relative_to(root)),
        "mode": stat.S_IMODE(path.stat().st_mode),
        "sha256": hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None,
    })
pathlib.Path(sys.argv[2]).write_text(json.dumps(entries, sort_keys=True), encoding="utf-8")
PY
if scripts/build/prepare-relation-pilot.sh \
  --binary "$tmp_bin" \
  --packet-key-file "$tmp_dir/relation-key.hex" --packet-key-id relation-pilot-package-packet-key \
  --qualification-key-file "$tmp_dir/relation-package-qualification-key.hex" --qualification-key-id relation-pilot-package-qualification-key \
  --package-root "$relation_pilot_package" \
  --bundle-created-at 2026-08-09T19:00:00Z \
  --readiness-prepared-at 2026-08-09T19:01:00Z \
  --dossier-prepared-at 2026-08-09T19:02:00Z > /dev/null 2>&1; then
  echo "relation pilot package preparation replaced an existing package" >&2
  exit 1
fi
python3 - "$relation_pilot_package" "$tmp_dir/relation-pilot-package-before.json" "$tmp_dir/relation-pilot-package-prepare.log" "$tmp_dir/relation-pilot-package-verification.log" <<'PY'
import hashlib
import json
import pathlib
import stat
import sys

root = pathlib.Path(sys.argv[1])
expected = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
observed = []
for path in [root, *sorted(root.rglob("*"))]:
    observed.append({
        "path": "." if path == root else str(path.relative_to(root)),
        "mode": stat.S_IMODE(path.stat().st_mode),
        "sha256": hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None,
    })
if observed != expected:
    raise SystemExit("existing relation pilot package changed after refused replacement")
summary = json.loads((root / "package-summary.json").read_text(encoding="utf-8"))
if summary["packets"] != 8 or len(summary["families"]) != 8:
    raise SystemExit("prepared relation pilot package lost packet or family coverage")
if summary["owner_inspection_status"] != "not_completed" or summary["human_study_status"] != "not_run" or summary["external_action_status"] != "not_authorized":
    raise SystemExit("prepared relation pilot package fabricated inspection, study, or authorization state")
if summary["launch_status"] != "not_launchable_pending_owner_inspection_and_authorization":
    raise SystemExit("prepared relation pilot package fabricated launch readiness")
prepare_log = pathlib.Path(sys.argv[3]).read_text(encoding="utf-8")
verify_log = pathlib.Path(sys.argv[4]).read_text(encoding="utf-8")
if "Owner inspection: not_completed" not in prepare_log or "providers=not_invoked" not in verify_log:
    raise SystemExit("relation pilot package logs lost their claim or provider boundary")
PY
python3 - "$qualification_key_file" "$tmp_dir" <<'PY'
import json
import os
import pathlib
import sys

key = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
root = pathlib.Path(sys.argv[2])
responses = [
    {"case_id": answer["case_id"], "observations": answer["observations"], "reason_codes": answer["reason_codes"]}
    for answer in key["answers"]
]
(root / "qualification-responses.json").write_text(json.dumps(responses, separators=(",", ":")), encoding="utf-8")
for name, value in (("assignment-seed-a.hex", "11" * 32), ("assignment-seed-b.hex", "22" * 32)):
    path = root / name
    path.write_text(value + "\n", encoding="ascii")
    os.chmod(path, 0o600)
packets = [json.loads((root / name).read_text(encoding="utf-8")) for name in ("pair-packet.json", "trajectory-packet.json")]
mappings = [json.loads(path.read_text(encoding="utf-8")) for path in sorted((root / "private-vault" / "mappings").glob("*.json"))]
(root / "packets.json").write_text(json.dumps(packets, separators=(",", ":")), encoding="utf-8")
(root / "mappings.json").write_text(json.dumps(mappings, separators=(",", ":")), encoding="utf-8")
os.chmod(root / "mappings.json", 0o600)
PY
for reviewer in a b; do
  reviewer_index=1
  if [[ "$reviewer" == b ]]; then reviewer_index=2; fi
  "$tmp_bin" relation qualify \
    --qualification-set @"$tmp_dir/qualification-set.json" \
    --answer-key @"$qualification_key_file" \
    --responses @"$tmp_dir/qualification-responses.json" \
    --reviewer-alias "synthetic-reviewer-$reviewer" \
    --qualified-at "2026-08-09T17:30:0${reviewer_index}Z" > "$tmp_dir/qualification-report-$reviewer.json"
  "$tmp_bin" relation reviewer \
    --alias "synthetic-reviewer-$reviewer" --role primary \
    --consented-at "2026-08-09T17:00:0${reviewer_index}Z" \
    --independence-attested --authorship-policy-accepted --contact-held-privately > "$tmp_dir/reviewer-$reviewer.json"
  "$tmp_bin" relation validate --type qualification-report --document @"$tmp_dir/qualification-report-$reviewer.json" > /dev/null
  "$tmp_bin" relation validate --type reviewer-record --document @"$tmp_dir/reviewer-$reviewer.json" > /dev/null
done
"$tmp_bin" relation bundle \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --sample-digest "$(python3 -c 'import json; print(json.load(open("eval/governance/relation-pilot-sample-v1.json"))["digest"])')" \
  --data-role development_pilot --packets @"$tmp_dir/packets.json" --mappings @"$tmp_dir/mappings.json" \
  --qualification-set @"$tmp_dir/qualification-set.json" --handbook @"$tmp_dir/reviewer-handbook.json" \
  --created-at 2026-08-09T18:00:00Z > "$tmp_dir/review-bundle.json"
"$tmp_bin" relation validate --type review-bundle --document @"$tmp_dir/review-bundle.json" > /dev/null
for reviewer in a b; do
  slot=1
  if [[ "$reviewer" == b ]]; then slot=2; fi
  "$tmp_bin" relation assign-primary \
    --bundle @"$tmp_dir/review-bundle.json" --mappings @"$tmp_dir/mappings.json" \
    --reviewer @"$tmp_dir/reviewer-$reviewer.json" --qualification @"$tmp_dir/qualification-report-$reviewer.json" \
    --slot "$slot" --seed-file "$tmp_dir/assignment-seed-$reviewer.hex" \
    --assigned-at "2026-08-09T18:10:0${slot}Z" > "$tmp_dir/assignment-$reviewer.json"
  "$tmp_bin" relation kit \
    --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-$reviewer.json" \
    --handbook @"$tmp_dir/reviewer-handbook.json" --generated-at "2026-08-09T18:20:0${slot}Z" > "$tmp_dir/reviewer-kit-$reviewer.json"
  "$tmp_bin" relation verify-kit \
    --kit @"$tmp_dir/reviewer-kit-$reviewer.json" --bundle @"$tmp_dir/review-bundle.json" \
    --assignment @"$tmp_dir/assignment-$reviewer.json" > /dev/null
  "$tmp_bin" relation render-kit \
    --kit @"$tmp_dir/reviewer-kit-$reviewer.json" --bundle @"$tmp_dir/review-bundle.json" \
    --assignment @"$tmp_dir/assignment-$reviewer.json" > "$tmp_dir/reviewer-kit-$reviewer.md"
  "$tmp_bin" relation validate --type review-assignment --document @"$tmp_dir/assignment-$reviewer.json" > /dev/null
  "$tmp_bin" relation validate --type reviewer-kit --document @"$tmp_dir/reviewer-kit-$reviewer.json" > /dev/null
done
python3 - "$tmp_dir" <<'PY'
import json
import os
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
assignments = {suffix: json.loads((root / f"assignment-{suffix}.json").read_text(encoding="utf-8")) for suffix in ("a", "b")}
all_packet_ids = sorted(assignments["a"]["packet_ids"])
disagreement_packet = all_packet_ids[-1]

def observations(semantic):
    return [
        {"axis": "causal_integrity_preservation", "rating": "not_applicable"},
        {"axis": "evidence_strength", "rating": "equal"},
        {"axis": "executable_outcome_support", "rating": "not_applicable"},
        {"axis": "information_sufficiency", "rating": "sufficient"},
        {"axis": "presentation_equivalence", "rating": "equal"},
        {"axis": "semantic_task_quality", "rating": semantic},
        {"axis": "untrusted_content_authority", "rating": "not_applicable"},
    ]

for suffix, assignment in assignments.items():
    for index, packet_id in enumerate(assignment["packet_ids"]):
        semantic = "right" if suffix == "b" and packet_id == disagreement_packet else "equal"
        reason = "task_quality_differs" if semantic == "right" else "no_judgment_relevant_change"
        draft = {
            "packet_id": packet_id,
            "observations": observations(semantic),
            "reason_codes": [reason],
            "submitted_at": f"2026-08-09T18:21:{10 + index:02d}Z",
            "revision_reason": "synthetic provider-free audit response",
        }
        (root / f"judgment-draft-{suffix}-{index}.json").write_text(json.dumps(draft, separators=(",", ":")), encoding="utf-8")

first = assignments["a"]["packet_ids"][0]
initial = {
    "packet_id": first,
    "observations": observations("left"),
    "reason_codes": ["task_quality_differs"],
    "submitted_at": "2026-08-09T18:21:01Z",
    "revision_reason": "synthetic initial transcription",
}
(root / "judgment-draft-a-0-initial.json").write_text(json.dumps(initial, separators=(",", ":")), encoding="utf-8")
(root / "assignment-seed-c.hex").write_text("33" * 32 + "\n", encoding="ascii")
os.chmod(root / "assignment-seed-c.hex", 0o600)
(root / "disagreement-packet-id.txt").write_text(disagreement_packet + "\n", encoding="utf-8")
PY
for reviewer in a b; do
  for index in 0 1; do
    if [[ "$reviewer" == a && "$index" == 0 ]]; then
      "$tmp_bin" relation judgment \
        --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-a.json" \
        --draft @"$tmp_dir/judgment-draft-a-0-initial.json" > "$tmp_dir/judgment-a-0-initial.json"
      "$tmp_bin" relation judgment \
        --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-a.json" \
        --draft @"$tmp_dir/judgment-draft-a-0.json" --parent @"$tmp_dir/judgment-a-0-initial.json" > "$tmp_dir/judgment-a-0.json"
    else
      "$tmp_bin" relation judgment \
        --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-$reviewer.json" \
        --draft @"$tmp_dir/judgment-draft-$reviewer-$index.json" > "$tmp_dir/judgment-$reviewer-$index.json"
    fi
    "$tmp_bin" relation validate --type pair-judgment --document @"$tmp_dir/judgment-$reviewer-$index.json" > /dev/null
  done
done
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
for suffix in ("a", "b"):
    judgments = [json.loads((root / f"judgment-{suffix}-{index}.json").read_text(encoding="utf-8")) for index in (0, 1)]
    (root / f"judgments-{suffix}.json").write_text(json.dumps(judgments, separators=(",", ":")), encoding="utf-8")
(root / "partial-judgments.json").write_text(json.dumps([json.loads((root / "judgment-a-0.json").read_text(encoding="utf-8"))], separators=(",", ":")), encoding="utf-8")
PY
if "$tmp_bin" relation judgment-batch \
  --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-a.json" \
  --judgments @"$tmp_dir/partial-judgments.json" --committed-at 2026-08-09T18:30:00Z > /dev/null 2>&1; then
  echo "relation judgment batch accepted partial assignment coverage" >&2
  exit 1
fi
for reviewer in a b; do
  slot=1
  if [[ "$reviewer" == b ]]; then slot=2; fi
  "$tmp_bin" relation judgment-batch \
    --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-$reviewer.json" \
    --judgments @"$tmp_dir/judgments-$reviewer.json" --committed-at "2026-08-09T18:30:0${slot}Z" > "$tmp_dir/judgment-batch-$reviewer.json"
  "$tmp_bin" relation validate --type judgment-batch --document @"$tmp_dir/judgment-batch-$reviewer.json" > /dev/null
done
if "$tmp_bin" relation analyze-ambiguity \
  --bundle @"$tmp_dir/review-bundle.json" \
  --left-assignment @"$tmp_dir/assignment-a.json" --left-batch @"$tmp_dir/judgment-batch-a.json" \
  --right-assignment @"$tmp_dir/assignment-b.json" --right-batch @"$tmp_dir/judgment-batch-b.json" \
  --analyzed-at 2026-08-09T18:30:02Z > /dev/null 2>&1; then
  echo "relation prereveal ambiguity analysis accepted a non-postcommit time" >&2
  exit 1
fi
"$tmp_bin" relation analyze-ambiguity \
  --bundle @"$tmp_dir/review-bundle.json" \
  --left-assignment @"$tmp_dir/assignment-a.json" --left-batch @"$tmp_dir/judgment-batch-a.json" \
  --right-assignment @"$tmp_dir/assignment-b.json" --right-batch @"$tmp_dir/judgment-batch-b.json" \
  --analyzed-at 2026-08-09T18:31:00Z > "$tmp_dir/prereveal-ambiguity.json"
"$tmp_bin" relation validate --type prereveal-ambiguity --document @"$tmp_dir/prereveal-ambiguity.json" > /dev/null
"$tmp_bin" relation qualify \
  --qualification-set @"$tmp_dir/qualification-set.json" --answer-key @"$qualification_key_file" \
  --responses @"$tmp_dir/qualification-responses.json" --reviewer-alias synthetic-reviewer-c \
  --qualified-at 2026-08-09T17:30:03Z > "$tmp_dir/qualification-report-c.json"
"$tmp_bin" relation reviewer \
  --alias synthetic-reviewer-c --role tie_break --consented-at 2026-08-09T17:00:03Z \
  --independence-attested --authorship-policy-accepted --contact-held-privately > "$tmp_dir/reviewer-c.json"
"$tmp_bin" relation assign-tie \
  --bundle @"$tmp_dir/review-bundle.json" --mappings @"$tmp_dir/mappings.json" \
  --reviewer @"$tmp_dir/reviewer-c.json" --qualification @"$tmp_dir/qualification-report-c.json" \
  --ambiguity @"$tmp_dir/prereveal-ambiguity.json" \
  --left-assignment @"$tmp_dir/assignment-a.json" --left-batch @"$tmp_dir/judgment-batch-a.json" \
  --right-assignment @"$tmp_dir/assignment-b.json" --right-batch @"$tmp_dir/judgment-batch-b.json" \
  --seed-file "$tmp_dir/assignment-seed-c.hex" --assigned-at 2026-08-09T18:32:00Z > "$tmp_dir/assignment-c.json"
"$tmp_bin" relation kit \
  --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-c.json" \
  --handbook @"$tmp_dir/reviewer-handbook.json" --generated-at 2026-08-09T18:33:00Z > "$tmp_dir/reviewer-kit-c.json"
"$tmp_bin" relation verify-kit \
  --kit @"$tmp_dir/reviewer-kit-c.json" --bundle @"$tmp_dir/review-bundle.json" \
  --assignment @"$tmp_dir/assignment-c.json" > /dev/null
python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
tie = json.loads((root / "assignment-c.json").read_text(encoding="utf-8"))
observations = [
    {"axis": "causal_integrity_preservation", "rating": "not_applicable"},
    {"axis": "evidence_strength", "rating": "equal"},
    {"axis": "executable_outcome_support", "rating": "not_applicable"},
    {"axis": "information_sufficiency", "rating": "sufficient"},
    {"axis": "presentation_equivalence", "rating": "equal"},
    {"axis": "semantic_task_quality", "rating": "equal"},
    {"axis": "untrusted_content_authority", "rating": "not_applicable"},
]
draft = {"packet_id": tie["packet_ids"][0], "observations": observations, "reason_codes": ["no_judgment_relevant_change"], "submitted_at": "2026-08-09T18:34:00Z", "revision_reason": "synthetic tie-break response"}
(root / "judgment-draft-c.json").write_text(json.dumps(draft, separators=(",", ":")), encoding="utf-8")
for suffix in ("a", "b"):
    assignment = json.loads((root / f"assignment-{suffix}.json").read_text(encoding="utf-8"))
    drafts = [{"packet_id": packet_id, "family_guess": "unknown", "direction_guess": "unknown", "source_condition_guess": "unknown", "recognized_task": False, "task_identity_guess": "unknown", "recognition_basis": "none", "confidence": 0, "submitted_at": "2026-08-09T18:36:00Z"} for packet_id in assignment["packet_ids"]]
    (root / f"probe-drafts-{suffix}.json").write_text(json.dumps(drafts, separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" relation judgment \
  --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-c.json" \
  --draft @"$tmp_dir/judgment-draft-c.json" > "$tmp_dir/judgment-c.json"
python3 - "$tmp_dir/judgment-c.json" "$tmp_dir/judgments-c.json" <<'PY'
import json
import pathlib
import sys
pathlib.Path(sys.argv[2]).write_text(json.dumps([json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))], separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" relation judgment-batch \
  --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-c.json" \
  --judgments @"$tmp_dir/judgments-c.json" --committed-at 2026-08-09T18:35:00Z > "$tmp_dir/judgment-batch-c.json"
for reviewer in a b; do
  slot=1
  if [[ "$reviewer" == b ]]; then slot=2; fi
  "$tmp_bin" relation probe-batch \
    --bundle @"$tmp_dir/review-bundle.json" --assignment @"$tmp_dir/assignment-$reviewer.json" \
    --judgment-batch @"$tmp_dir/judgment-batch-$reviewer.json" --drafts @"$tmp_dir/probe-drafts-$reviewer.json" \
    --committed-at "2026-08-09T18:37:0${slot}Z" > "$tmp_dir/probe-batch-$reviewer.json"
  "$tmp_bin" relation validate --type condition-probe-batch --document @"$tmp_dir/probe-batch-$reviewer.json" > /dev/null
done
if "$tmp_bin" relation reveal \
  --bundle @"$tmp_dir/review-bundle.json" --mappings @"$tmp_dir/mappings.json" \
  --left-assignment @"$tmp_dir/assignment-a.json" --left-batch @"$tmp_dir/judgment-batch-a.json" --left-probes @"$tmp_dir/probe-batch-a.json" --left-seed-file "$tmp_dir/assignment-seed-a.hex" \
  --right-assignment @"$tmp_dir/assignment-b.json" --right-batch @"$tmp_dir/judgment-batch-b.json" --right-probes @"$tmp_dir/probe-batch-b.json" --right-seed-file "$tmp_dir/assignment-seed-b.hex" \
  --ambiguity @"$tmp_dir/prereveal-ambiguity.json" --tie-assignment @"$tmp_dir/assignment-c.json" --tie-batch @"$tmp_dir/judgment-batch-c.json" --tie-seed-file "$tmp_dir/assignment-seed-c.hex" \
  --revealed-at 2026-08-09T18:37:02Z --revealed-by synthetic-study-owner > /dev/null 2>&1; then
  echo "relation mapping reveal accepted a non-postcommit time" >&2
  exit 1
fi
"$tmp_bin" relation reveal \
  --bundle @"$tmp_dir/review-bundle.json" --mappings @"$tmp_dir/mappings.json" \
  --left-assignment @"$tmp_dir/assignment-a.json" --left-batch @"$tmp_dir/judgment-batch-a.json" --left-probes @"$tmp_dir/probe-batch-a.json" --left-seed-file "$tmp_dir/assignment-seed-a.hex" \
  --right-assignment @"$tmp_dir/assignment-b.json" --right-batch @"$tmp_dir/judgment-batch-b.json" --right-probes @"$tmp_dir/probe-batch-b.json" --right-seed-file "$tmp_dir/assignment-seed-b.hex" \
  --ambiguity @"$tmp_dir/prereveal-ambiguity.json" --tie-assignment @"$tmp_dir/assignment-c.json" --tie-batch @"$tmp_dir/judgment-batch-c.json" --tie-seed-file "$tmp_dir/assignment-seed-c.hex" \
  --revealed-at 2026-08-09T18:39:00Z --revealed-by synthetic-study-owner > "$tmp_dir/mapping-reveal.json"
"$tmp_bin" relation validate --type mapping-reveal --document @"$tmp_dir/mapping-reveal.json" > /dev/null
"$tmp_bin" relation compare \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --bundle @"$tmp_dir/review-bundle.json" --mappings @"$tmp_dir/mappings.json" \
  --reveal @"$tmp_dir/mapping-reveal.json" --ambiguity @"$tmp_dir/prereveal-ambiguity.json" \
  --left-assignment @"$tmp_dir/assignment-a.json" --left-batch @"$tmp_dir/judgment-batch-a.json" --left-probes @"$tmp_dir/probe-batch-a.json" \
  --right-assignment @"$tmp_dir/assignment-b.json" --right-batch @"$tmp_dir/judgment-batch-b.json" --right-probes @"$tmp_dir/probe-batch-b.json" \
  --tie-assignment @"$tmp_dir/assignment-c.json" --tie-batch @"$tmp_dir/judgment-batch-c.json" \
  --completed-at 2026-08-09T18:40:00Z > "$tmp_dir/formal-human-result.json"
python3 - "$tmp_dir/formal-human-result.json" "$tmp_dir/formal-human-comparison.json" "$tmp_dir/relation-resolutions.json" <<'PY'
import json
import pathlib
import sys

result = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
pathlib.Path(sys.argv[2]).write_text(json.dumps(result["comparison"], separators=(",", ":")), encoding="utf-8")
pathlib.Path(sys.argv[3]).write_text(json.dumps(result["resolutions"], separators=(",", ":")), encoding="utf-8")
PY
"$tmp_bin" relation validate --type formal-human-comparison --document @"$tmp_dir/formal-human-comparison.json" > /dev/null
python3 - "$tmp_dir/relation-resolutions.json" "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[2])
for index, resolution in enumerate(json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))):
    (root / f"relation-resolution-{index}.json").write_text(json.dumps(resolution, separators=(",", ":")), encoding="utf-8")
PY
for resolution_file in "$tmp_dir"/relation-resolution-*.json; do
  "$tmp_bin" relation validate --type relation-resolution --document @"$resolution_file" > /dev/null
done
"$tmp_bin" relation terminal-ledger \
  --bundle @"$tmp_dir/review-bundle.json" --reveal @"$tmp_dir/mapping-reveal.json" \
  --ambiguity @"$tmp_dir/prereveal-ambiguity.json" --comparison @"$tmp_dir/formal-human-comparison.json" \
  --resolutions @"$tmp_dir/relation-resolutions.json" --completed-at 2026-08-09T18:41:00Z > "$tmp_dir/terminal-relation-ledger.json"
"$tmp_bin" relation validate --type terminal-ledger --document @"$tmp_dir/terminal-relation-ledger.json" > /dev/null
if "$tmp_bin" outcome validate --type adjudication-ledger --document @"$tmp_dir/terminal-relation-ledger.json" > /dev/null 2>&1; then
  echo "outcome validation accepted a controlled-relation terminal ledger" >&2
  exit 1
fi
python3 - "$tmp_dir/formal-human-comparison.json" "$tmp_dir/tampered-comparison.json" <<'PY'
import json
import pathlib
import sys

comparison = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
comparison["task_group_denominators"][0]["denominator"] -= 1
comparison["task_group_denominators"][0]["supports"] -= 1
pathlib.Path(sys.argv[2]).write_text(json.dumps(comparison, separators=(",", ":")), encoding="utf-8")
PY
if "$tmp_bin" relation validate --type formal-human-comparison --document @"$tmp_dir/tampered-comparison.json" > /dev/null 2>&1; then
  echo "relation formal-human comparison accepted denominator deletion" >&2
  exit 1
fi
issued_at=$(python3 -c 'import json; print(json.load(open("eval/governance/relation-study-amendment-v1.json", encoding="utf-8"))["issued_at"])')
"$tmp_bin" relation study-amendment \
  --plan @eval/governance/relation-audit-plan-v1.json \
  --pilot-sample @"$tmp_dir/pilot-sample.json" \
  --primary-sample @"$tmp_dir/primary-sample.json" \
  --issued-at "$issued_at" > "$tmp_dir/study-amendment.json"
"$tmp_bin" relation validate --type study-amendment --document @"$tmp_dir/study-amendment.json" > "$tmp_dir/amendment-validation.json"

python3 - "$tmp_dir" "$schema_count" <<'PY'
import hashlib
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
schema_count = int(sys.argv[2])

def load(name):
    return json.loads((root / name).read_text(encoding="utf-8"))

plan = load("plan.json")
plan_validation = load("plan-validation.json")
release = load("release.json")
sample = load("primary-sample.json")
sample_validation = load("sample-validation.json")
pilot = load("pilot-sample.json")
pilot_validation = load("pilot-validation.json")
pair_replay = load("pair-replay.json")
trajectory_replay = load("trajectory-replay.json")
pair_material = load("pair-material.json")
trajectory_material = load("trajectory-material.json")
pair_packet = load("pair-packet.json")
trajectory_packet = load("trajectory-packet.json")
qualification_set = load("qualification-set.json")
handbook = load("reviewer-handbook.json")
review_bundle = load("review-bundle.json")
assignments = [load("assignment-a.json"), load("assignment-b.json")]
reviewer_kits = [load("reviewer-kit-a.json"), load("reviewer-kit-b.json")]
judgment_batches = [load("judgment-batch-a.json"), load("judgment-batch-b.json")]
initial_judgment = load("judgment-a-0-initial.json")
revised_judgment = load("judgment-a-0.json")
ambiguity = load("prereveal-ambiguity.json")
tie_assignment = load("assignment-c.json")
tie_kit = load("reviewer-kit-c.json")
tie_batch = load("judgment-batch-c.json")
probe_batches = [load("probe-batch-a.json"), load("probe-batch-b.json")]
mapping_reveal = load("mapping-reveal.json")
pilot_readiness = load("pilot-readiness.json")
pilot_inspection_receipt = load("pilot-inspection-receipt.json")
pilot_inspection = json.loads((root / "pilot-inspection-vault" / pilot_inspection_receipt["relative_path"]).read_text(encoding="utf-8"))
pilot_inspection_verification = load("pilot-inspection-verification.json")
pilot_inspection_workbook = (root / "pilot-inspection-workbook.md").read_text(encoding="utf-8")
formal_human = load("formal-human-comparison.json")
resolutions = load("relation-resolutions.json")
terminal_ledger = load("terminal-relation-ledger.json")
amendment = load("study-amendment.json")
amendment_validation = load("amendment-validation.json")
translations = {name: load(f"{name}-translation.json") for name in ("support", "contradict", "unresolved")}
governed_plan = json.loads(pathlib.Path("eval/governance/relation-audit-plan-v1.json").read_text(encoding="utf-8"))
governed_sample = json.loads(pathlib.Path("eval/governance/relation-primary-sample-v1.json").read_text(encoding="utf-8"))
governed_pilot = json.loads(pathlib.Path("eval/governance/relation-pilot-sample-v1.json").read_text(encoding="utf-8"))
governed_amendment = json.loads(pathlib.Path("eval/governance/relation-study-amendment-v1.json").read_text(encoding="utf-8"))
mapping_paths = sorted((root / "private-vault" / "mappings").glob("*.json"))
mappings = [json.loads(path.read_text(encoding="utf-8")) for path in mapping_paths]
mapping_by_packet = {mapping["packet_id"]: mapping for mapping in mappings}

if plan != governed_plan or plan_validation != {"valid": True, "type": "plan", "digest": governed_plan["digest"]}:
    raise SystemExit("governed relation audit plan did not reproduce exactly")
if sample != governed_sample or sample_validation != {"valid": True, "type": "primary-sample", "digest": governed_sample["digest"]}:
    raise SystemExit("governed relation primary sample did not reproduce exactly")
if pilot != governed_pilot or pilot_validation != {"valid": True, "type": "pilot-sample", "digest": governed_pilot["digest"]}:
    raise SystemExit("governed relation development pilot did not reproduce exactly")
if amendment != governed_amendment or amendment_validation != {"valid": True, "type": "study-amendment", "digest": governed_amendment["digest"]}:
    raise SystemExit("governed relation study amendment did not reproduce exactly")
if plan["review_objective"] != "controlled_relation" or plan["external_action_status"] != "not_authorized":
    raise SystemExit("relation plan weakened its objective or external-action boundary")
if len(plan["axes"]) != 7 or len(plan["families"]) != 8 or any(not row["support_all"] or not row["contradict_any"] for row in plan["families"]):
    raise SystemExit("relation plan lost an axis, family, or deterministic translation branch")
if len(plan["reason_codes"]) != 12 or {name: value["state"] for name, value in translations.items()} != {"support": "supports", "contradict": "contradicts", "unresolved": "unresolved"}:
    raise SystemExit("relation plan or executable translation lost its reason or terminal-state contract")
if amendment["empirical_status"] != "not_run" or amendment["external_action_status"] != "not_authorized" or amendment["primary"]["effective_task_groups"] != 28:
    raise SystemExit("relation amendment fabricated empirical work or lost its task-group design")
if amendment["pilot_sample_digest"] != pilot["digest"] or amendment["primary_sample_digest"] != sample["digest"]:
    raise SystemExit("relation amendment does not bind the exact pilot and primary commitments")
if pilot_readiness["pilot_sample_digest"] != pilot["digest"] or pilot_readiness["packets"] != 8 or len(pilot_readiness["packet_checks"]) != 8:
    raise SystemExit("relation pilot readiness does not bind all eight governed pilot packets")
if pilot_readiness["technical_status"] != "structurally_ready_for_owner_semantic_review" or pilot_readiness["semantic_inspection_status"] != "requires_owner_manual_inspection":
    raise SystemExit("relation pilot readiness overstates semantic inspection")
if pilot_readiness["external_action_status"] != "not_authorized" or len({check["family"] for check in pilot_readiness["packet_checks"]}) != 8:
    raise SystemExit("relation pilot readiness weakened authorization or family coverage")
if pilot_inspection["readiness_digest"] != pilot_readiness["digest"] or pilot_inspection["packets"] != 8 or len(pilot_inspection["decisions"]) != 8:
    raise SystemExit("relation pilot inspection does not bind all eight readiness packets")
if pilot_inspection["overall_status"] != "passed" or pilot_inspection["accepted"] != 8 or pilot_inspection["revision_required"] or pilot_inspection["unresolved"]:
    raise SystemExit("relation synthetic pilot inspection fixture did not reproduce its exact decision counts")
if pilot_inspection["human_study_status"] != "not_run" or pilot_inspection["external_action_status"] != "not_authorized":
    raise SystemExit("relation pilot inspection fabricated human-study evidence or authorization")
if pilot_inspection_receipt != {"published": True, "digest": pilot_inspection["digest"], "relative_path": f"pilot-inspections/{pilot_inspection['digest']}.json", "overall_status": "passed", "human_study_status": "not_run", "external_action_status": "not_authorized"}:
    raise SystemExit("relation pilot inspection did not use exclusive owner-only custody")
if pilot_inspection_verification != {"valid": True, "inspection_digest": pilot_inspection["digest"], "readiness_digest": pilot_readiness["digest"]}:
    raise SystemExit("relation pilot inspection was not independently rebound to readiness, bundle, and mappings")
if "OWNER-ONLY RESTRICTED SURFACE" not in pilot_inspection_workbook or "not a reviewer kit, human-study result, or authorization" not in pilot_inspection_workbook:
    raise SystemExit("relation pilot inspection workbook lost its private or claim boundary")
if any(check["packet_id"] not in pilot_inspection_workbook or check["family"] not in pilot_inspection_workbook for check in pilot_readiness["packet_checks"]):
    raise SystemExit("relation pilot inspection workbook omits a packet or hidden family")
if not (0.10 < amendment["inference"]["zero_contradiction_upper_bound"] < 0.11) or amendment["inference"]["family_analysis_role"].startswith("descriptive") is False:
    raise SystemExit("relation amendment overstates its zero-defect or family-level resolution")
if sample["source_corpus_digest"] != release["digest"] or sample["selected_cases"] != 31:
    raise SystemExit("relation primary sample does not bind the exact 31-case release selection")
if sample["unique_source_ids"] != 34 or sample["unique_task_groups"] != 28 or sample["trajectory_pair_units"] != 26 or sample["candidate_order_units"] != 5:
    raise SystemExit("relation primary sample unit or lineage denominators changed")
if sample["selection_digest"] != "4261c63feb43a3ca0531cf20bf6cd773fb8bbb421f1732dd836719d309e346ba":
    raise SystemExit("relation primary selection identity changed")
if len(set(sample["bindings"].values())) != 10 or any(len(value) != 64 for value in sample["bindings"].values()):
    raise SystemExit("relation primary sample lost a deep binding commitment")
if pilot["selected_cases"] != 8 or pilot["unique_source_ids"] != 9 or pilot["unique_task_groups"] != 8 or pilot["unique_lineage_clusters"] != 8 or pilot["primary_overlap"] != 0:
    raise SystemExit("relation development pilot size, unit, or overlap contract changed")
if len(set(pilot["bindings"].values())) != 10 or any(len(value) != 64 for value in pilot["bindings"].values()):
    raise SystemExit("relation development pilot lost a deep binding commitment")

selected = [case for case in release["cases"] if case["manifest"]["review"]["required"]]
identities = sorted(case["id"] + "\0" + case["blind_review_packet"]["digest"] for case in selected)
selection_digest = hashlib.sha256("\0".join(identities).encode("utf-8")).hexdigest()
if len(selected) != sample["selected_cases"] or selection_digest != sample["selection_digest"]:
    raise SystemExit("independent relation selection reproduction disagrees with the Go builder")
if any(case["family"] == "candidate_order_reversal" and len(case["source_ids"]) != 2 for case in selected):
    raise SystemExit("candidate-order relation unit collapsed below pair-of-pairs source coverage")
if any(case["family"] != "candidate_order_reversal" and len(case["source_ids"]) != 1 for case in selected):
    raise SystemExit("trajectory-level relation unit has the wrong source arity")

source_by_id = {source["id"]: source for source in release["sources"]}
primary_sources = {source_id for case in selected for source_id in case["source_ids"]}
primary_groups = {case["manifest"]["split_group_id"] for case in selected}
primary_lineages = {source_by_id[source_id]["lineage_cluster_id"] for source_id in primary_sources}
pilot_sources = {source_id for case in pilot["cases"] for source_id in case["source_ids"]}
pilot_groups = {case["task_group_id"] for case in pilot["cases"]}
pilot_lineages = {lineage_id for case in pilot["cases"] for lineage_id in case["lineage_cluster_ids"]}
if primary_sources & pilot_sources or primary_groups & pilot_groups or primary_lineages & pilot_lineages:
    raise SystemExit("relation development pilot overlaps the primary sample by source, task group, or lineage")
if len({case["family"] for case in pilot["cases"]}) != 8 or sum(case["unit"] == "candidate_pair_orderings" for case in pilot["cases"]) != 1:
    raise SystemExit("relation development pilot lost one-family coverage or its pair-of-pairs unit")
case_by_id = {case["id"]: case for case in release["cases"]}
for receipt, unit, arity in ((pair_replay, "candidate_pair_orderings", 2), (trajectory_replay, "trajectory_pair", 1)):
    case = case_by_id[receipt["case_id"]]
    if receipt["unit"] != unit or receipt["replay_status"] != "exact" or receipt["external_action_status"] != "not_authorized":
        raise SystemExit("relation replay receipt lost its unit, exactness, or authorization boundary")
    if receipt["source_ids"] != case["source_ids"] or receipt["manifest_digest"] != case["manifest"]["digest"] or receipt["blind_packet_digest"] != case["blind_review_packet"]["digest"]:
        raise SystemExit("relation replay receipt does not bind its frozen source, manifest, or packet")
    if len(receipt["original_trajectory_digests"]) != arity or len(receipt["transformed_trajectory_digests"]) != arity:
        raise SystemExit("relation replay receipt has the wrong material arity")
if pair_replay["original_trajectory_digests"] != list(reversed(pair_replay["transformed_trajectory_digests"])):
    raise SystemExit("candidate pair-of-pairs replay is not an exact order reversal")
for material, unit, arity in ((pair_material, "candidate_pair_orderings", 2), (trajectory_material, "trajectory_pair", 1)):
    if material["unit"] != unit or len(material["original"]) != arity or len(material["transformed"]) != arity or material["external_action_status"] != "not_authorized":
        raise SystemExit("relation material lost its unit, arity, or authorization boundary")
    if not material["task_requirement"].strip() or len(material["limitations"]) != 3:
        raise SystemExit("relation material lacks coherent task context or limitations")
    for excerpt in material["original"] + material["transformed"]:
        if excerpt["redistribution"] != "reference_only" or excerpt["visibility"] != "restricted_reference_only" or excerpt["public_releasable"]:
            raise SystemExit("relation material upgraded reference-only evidence to public")
        if excerpt["omitted_events"] != excerpt["source_events"] - excerpt["retained_events"] or excerpt["evidence_selector"] != "evalwitness.relation-paired-evidence-selector.v1":
            raise SystemExit("relation material lost omission accounting or selector identity")
if [item["content_digest"] for item in pair_material["original"]] != list(reversed([item["content_digest"] for item in pair_material["transformed"]])):
    raise SystemExit("relation candidate-order excerpts are not exact reversals")

for material in (pair_material, trajectory_material):
    case = case_by_id[material["case_id"]]
    private_values = []
    for source_id in case["source_ids"]:
        source = source_by_id[source_id]
        private_values.extend([source_id, source["task_id"], source["repository_id"], source["source_family"], source["source_location"], source["source_revision"]])
    visible = material["task_requirement"] + "\n" + "\n".join(item["content"] for item in material["original"] + material["transformed"])
    if any(value and value.lower() in visible.lower() for value in private_values):
        raise SystemExit("relation restricted material leaks a frozen source identity")

for packet, material in ((pair_packet, pair_material), (trajectory_packet, trajectory_material)):
    mapping = mapping_by_packet.get(packet["packet_id"])
    if mapping is None or mapping["packet_digest"] != packet["digest"]:
        raise SystemExit("relation packet lacks its exact owner-only mapping binding")
    if packet["review_objective"] != "controlled_relation" or mapping["review_objective"] != "controlled_relation":
        raise SystemExit("relation packet or mapping lost objective typing")
    if packet["external_action_status"] != "not_authorized" or mapping["external_action_status"] != "not_authorized":
        raise SystemExit("relation packet or mapping weakened the external-action boundary")
    if packet["unit"] != material["unit"] or packet["task_requirement"] != material["task_requirement"]:
        raise SystemExit("relation packet changed its review unit or coherent task requirement")
    if len(packet["rubric_questions"]) != 7 or [side["position"] for side in packet["sides"]] != ["left", "right"]:
        raise SystemExit("relation packet lost the full neutral rubric or fixed visible positions")
    if pathlib.Path(mapping_paths[mappings.index(mapping)]).stem != mapping["digest"]:
        raise SystemExit("relation private mapping filename is not content-addressed")
    if mapping["digest"] in json.dumps(packet, sort_keys=True):
        raise SystemExit("relation packet embeds its owner-only mapping")

    forbidden_keys = {
        "case_id", "control", "expected_relation", "family", "manifest_digest", "mapping", "mutation_operator",
        "provider_identity", "regeneration_key", "reviewer_order_key", "source_ids", "source_revision",
        "source_trajectory_digest", "source_url", "split", "split_role", "validator", "verifier_confidence",
        "verifier_decision", "verifier_score", "witness_digest",
    }
    def keys(value):
        if isinstance(value, dict):
            return set(value) | set().union(*(keys(child) for child in value.values()), set())
        if isinstance(value, list):
            return set().union(*(keys(child) for child in value), set())
        return set()
    if keys(packet) & forbidden_keys:
        raise SystemExit("relation packet contains a forbidden private field")
    case = case_by_id[material["case_id"]]
    private_values = [case["id"], case["family"], case["manifest"]["expected_relation"], mapping["blinding_key_id"]]
    for source_id in case["source_ids"]:
        source = source_by_id[source_id]
        private_values.extend([source_id, source["task_id"], source["repository_id"], source["source_family"], source["source_location"], source["source_revision"]])
    encoded_packet = json.dumps(packet, sort_keys=True).lower()
    if any(value and value.lower() in encoded_packet for value in private_values):
        raise SystemExit("relation packet leaks a hidden case, family, relation, key, or source value")

    key = bytes.fromhex((root / "relation-key.hex").read_text(encoding="ascii").strip())
    def keyed(domain, *values):
        import hmac
        import struct
        payload = struct.pack(">Q", len(domain)) + domain.encode()
        for value in values:
            encoded = str(value).encode()
            payload += struct.pack(">Q", len(encoded)) + encoded
        return hmac.new(key, payload, hashlib.sha256).hexdigest()
    base = [mapping["plan_digest"], mapping["source_corpus_digest"], mapping["case_material_digest"], mapping["case_id"], mapping["blinding_key_id"]]
    domains = {row["purpose"]: row["domain"] for row in mapping["randomization_domains"]}
    if set(domains) != {"candidate_label", "evidence_slot_identity", "packet_id", "packet_order", "reviewer_assignment_order", "task_alias", "visible_side_identity"}:
        raise SystemExit("relation mapping lost an independent randomization domain")
    if packet["packet_id"] != "relation-packet-" + keyed(domains["packet_id"], *base):
        raise SystemExit("relation packet ID is not the exact domain-separated HMAC")
    if packet["task_alias"] != "relation-task-" + keyed(domains["task_alias"], mapping["plan_digest"], mapping["source_corpus_digest"], mapping["source_task_group_id"], mapping["blinding_key_id"]):
        raise SystemExit("relation task alias is not the exact domain-separated HMAC")
    if mapping["packet_order_key"] != keyed(domains["packet_order"], *base) or mapping["reviewer_order_key"] != keyed(domains["reviewer_assignment_order"], *base):
        raise SystemExit("relation packet or reviewer order key is not the exact domain-separated HMAC")
    if packet["packet_order_commitment"] != hashlib.sha256(mapping["packet_order_key"].encode()).hexdigest() or packet["reviewer_order_commitment"] != hashlib.sha256(mapping["reviewer_order_key"].encode()).hexdigest():
        raise SystemExit("relation public order commitment does not bind the private sort key")
    outputs = [packet["packet_id"].removeprefix("relation-packet-"), packet["task_alias"].removeprefix("relation-task-"), mapping["packet_order_key"], mapping["reviewer_order_key"]]
    seen_side_roles = set()
    for evidence in mapping["evidence_mappings"]:
        expected_side = "relation-side-" + keyed(domains["visible_side_identity"], *base, evidence["logical_side"])
        expected_slot = "relation-slot-" + keyed(domains["evidence_slot_identity"], *base, evidence["logical_side"], evidence["visible_evidence_index"], evidence["content_digest"])
        if evidence["side_alias"] != expected_side or evidence["slot_id"] != expected_slot:
            raise SystemExit("relation side or evidence slot is not the exact domain-separated HMAC")
        if evidence["logical_side"] not in seen_side_roles:
            outputs.append(evidence["side_alias"].removeprefix("relation-side-"))
            seen_side_roles.add(evidence["logical_side"])
        outputs.append(evidence["slot_id"].removeprefix("relation-slot-"))
        if packet["unit"] == "candidate_pair_orderings":
            expected_candidate = "relation-candidate-" + keyed(domains["candidate_label"], *base, evidence["logical_side"], evidence["visible_evidence_index"], evidence["source_candidate_index"], evidence["content_digest"])
            if evidence["candidate_label"] != expected_candidate:
                raise SystemExit("relation candidate label is not the exact domain-separated HMAC")
            outputs.append(evidence["candidate_label"].removeprefix("relation-candidate-"))
        elif "candidate_label" in evidence:
            raise SystemExit("trajectory relation mapping invented a candidate label")
    if len(outputs) != len(set(outputs)):
        raise SystemExit("relation blinding reused an output across independent randomization domains")

if [item["content_digest"] for item in pair_packet["sides"][0]["evidence"]] != list(reversed([item["content_digest"] for item in pair_packet["sides"][1]["evidence"]])):
    raise SystemExit("relation candidate packet collapsed its exact pair-of-pairs reversal")
if any("candidate_label" in item for side in trajectory_packet["sides"] for item in side["evidence"]):
    raise SystemExit("relation trajectory packet invented candidate labels")

qualification_public = json.dumps(qualification_set, sort_keys=True)
if len(qualification_set["cases"]) != 8 or len({case["competency"] for case in qualification_set["cases"]}) != 8:
    raise SystemExit("relation qualification lost one of eight distinct competencies")
if any(value in qualification_public for value in ("owner_only_answer_key", '"answers"', '"explanation"')):
    raise SystemExit("relation qualification set exposes its owner-only answers")
if qualification_set["mandatory_case_ids"] != ["relation-qualification-07", "relation-qualification-08"] or qualification_set["passing_score"] != 0.875:
    raise SystemExit("relation qualification weakened its ambiguity/order mandatory cases or passing score")
if handbook["review_objective"] != "controlled_relation" or handbook["qualification_set_digest"] != qualification_set["digest"] or len(handbook["axis_definitions"]) != 7 or len(handbook["reason_definitions"]) != 12:
    raise SystemExit("relation handbook lost objective, qualification, axis, or reason binding")
if review_bundle["ordering_protocol"] != "evalwitness.relation-private-packet-order.v1" or review_bundle["external_action_status"] != "not_authorized" or len(review_bundle["packets"]) != 2:
    raise SystemExit("relation review bundle lost private ordering, packet coverage, or authorization boundary")
bundle_packet_ids = {packet["packet_id"] for packet in review_bundle["packets"]}
if any(set(assignment["packet_ids"]) != bundle_packet_ids or not assignment["qualification"]["qualified"] or assignment["distribution_status"] != "planned_not_shared" for assignment in assignments):
    raise SystemExit("relation assignment lost packet coverage, qualification, or no-sharing boundary")
if assignments[0]["reviewer_slot"] != 1 or assignments[1]["reviewer_slot"] != 2 or assignments[0]["reviewer"]["reviewer_alias"] == assignments[1]["reviewer"]["reviewer_alias"]:
    raise SystemExit("relation primary assignments are not independent reviewer slots")
for assignment, suffix in zip(assignments, ("a", "b")):
    import hmac
    import struct
    seed = bytes.fromhex((root / f"assignment-seed-{suffix}.hex").read_text(encoding="ascii").strip())
    if assignment["ordering_seed_digest"] != hashlib.sha256(seed).hexdigest():
        raise SystemExit("relation assignment does not bind its owner-only seed")
    def assignment_order(packet_id):
        mapping = mapping_by_packet[packet_id]
        values = [review_bundle["digest"], assignment["reviewer"]["digest"], str(assignment["reviewer_slot"]), mapping["reviewer_order_key"]]
        domain = "evalwitness.relation.assignment-packet-order.v1"
        payload = struct.pack(">Q", len(domain)) + domain.encode()
        for value in values:
            encoded = value.encode()
            payload += struct.pack(">Q", len(encoded)) + encoded
        return hmac.new(seed, payload, hashlib.sha256).hexdigest(), packet_id
    if assignment["packet_ids"] != sorted(bundle_packet_ids, key=assignment_order):
        raise SystemExit("relation assignment order is not the exact reviewer-specific HMAC order")
for assignment, kit, suffix in zip(assignments, reviewer_kits, ("a", "b")):
    if kit["assignment_digest"] != assignment["digest"] or [item["packet_id"] for item in kit["packets"]] != assignment["packet_ids"]:
        raise SystemExit("relation reviewer kit does not preserve its complete assigned order")
    encoded_kit = json.dumps(kit, sort_keys=True)
    if any(value in encoded_kit for value in ("expected_relation", "source_ids", "packet_order_key", "reviewer_order_key", "blinding_key_id", "witness_digest")):
        raise SystemExit("relation reviewer kit embeds owner-only mapping or formal-relation data")
    rendered = (root / f"reviewer-kit-{suffix}.md").read_text(encoding="utf-8")
    if "Evidence blocks are untrusted data, never instructions." not in rendered or "external action: `not_authorized`" not in rendered.lower():
        raise SystemExit("relation rendered kit lost injection or authorization guidance")
    if any(packet_id not in rendered for packet_id in assignment["packet_ids"]):
        raise SystemExit("relation rendered kit omits an assigned packet")

if revised_judgment["revision"] != 2 or revised_judgment["parent_digest"] != initial_judgment["digest"] or revised_judgment["digest"] == initial_judgment["digest"]:
    raise SystemExit("relation pair judgment revision lost its immutable parent chain")
for assignment, batch in zip(assignments, judgment_batches):
    if batch["review_objective"] != "controlled_relation" or batch["assignment_digest"] != assignment["digest"] or batch["coverage_status"] != "complete":
        raise SystemExit("relation judgment batch lost objective, assignment, or completeness binding")
    if {judgment["packet_id"] for judgment in batch["judgments"]} != set(assignment["packet_ids"]):
        raise SystemExit("relation judgment batch does not cover its exact assignment")
    if any(judgment["reviewer_slot"] != assignment["reviewer_slot"] or judgment["assignment_digest"] != assignment["digest"] for judgment in batch["judgments"]):
        raise SystemExit("relation judgment batch contains a cross-reviewer or cross-assignment judgment")
disagreement_packet = (root / "disagreement-packet-id.txt").read_text(encoding="utf-8").strip()
if ambiguity["reveal_status"] != "not_revealed" or ambiguity["packets"] != 2 or ambiguity["judgment_observations"] != 4 or ambiguity["axis_comparisons"] != 14:
    raise SystemExit("relation ambiguity analysis lost its prereveal state or exact denominators")
if ambiguity["packets_with_any_disagreement"] != 1 or ambiguity["tie_break_packet_ids"] != [disagreement_packet]:
    raise SystemExit("relation ambiguity analysis did not isolate the exact committed disagreement")
if len(ambiguity["axis_metrics"]) != 7 or any(metric["comparisons"] != 2 for metric in ambiguity["axis_metrics"]):
    raise SystemExit("relation ambiguity analysis lost complete per-axis denominators")
if keys(ambiguity) & {"case_id", "family", "expected_relation", "mapping", "source_condition", "verifier_score", "validator_result"}:
    raise SystemExit("relation prereveal ambiguity analysis contains hidden or post-reveal fields")
if tie_assignment["purpose"] != "tie_break" or tie_assignment["reviewer_slot"] != 3 or tie_assignment["packet_ids"] != [disagreement_packet]:
    raise SystemExit("relation tie-break assignment escaped exact disagreement-only scope")
if tie_assignment["reviewer"]["reviewer_alias"] in {assignment["reviewer"]["reviewer_alias"] for assignment in assignments}:
    raise SystemExit("relation tie-break reviewer is not independent of both primaries")
if tie_assignment["distribution_status"] != "planned_not_shared" or tie_assignment["external_action_status"] != "not_authorized":
    raise SystemExit("relation tie-break assignment weakened the no-sharing boundary")
if tie_kit["assignment_digest"] != tie_assignment["digest"] or [item["packet_id"] for item in tie_kit["packets"]] != [disagreement_packet]:
    raise SystemExit("relation tie-break kit differs from the disagreement-only assignment")
if tie_batch["assignment_digest"] != tie_assignment["digest"] or [item["packet_id"] for item in tie_batch["judgments"]] != [disagreement_packet]:
    raise SystemExit("relation tie-break judgment commitment escaped disagreement-only scope")
for assignment, batch, probes in zip(assignments, judgment_batches, probe_batches):
    if probes["assignment_digest"] != assignment["digest"] or probes["judgment_batch_digest"] != batch["digest"] or len(probes["probes"]) != 2:
        raise SystemExit("relation condition probes do not bind complete post-label primary commitments")
    if any(probe["judgment_digest"] not in {judgment["digest"] for judgment in batch["judgments"]} for probe in probes["probes"]):
        raise SystemExit("relation condition probe does not bind an exact committed judgment")
if mapping_reveal["ambiguity_analysis_digest"] != ambiguity["digest"] or mapping_reveal["tie_break_batch_digest"] != tie_batch["digest"]:
    raise SystemExit("relation mapping reveal lost prereveal ambiguity or tie-break commitment")
if len(mapping_reveal["probe_batch_digests"]) != 2 or len(mapping_reveal["ordering_seeds"]) != 3 or len(mapping_reveal["mappings"]) != 2:
    raise SystemExit("relation mapping reveal is incomplete")
if mapping_reveal["external_action_status"] != "not_authorized" or mapping_reveal["review_objective"] != "controlled_relation":
    raise SystemExit("relation mapping reveal weakened objective or authorization custody")
if formal_human["mapping_reveal_digest"] != mapping_reveal["digest"] or formal_human["ambiguity_analysis_digest"] != ambiguity["digest"]:
    raise SystemExit("relation formal-human comparison lost reveal or prereveal custody")
if formal_human["packet_states"]["denominator"] != 2 or sum(formal_human["packet_states"][state] for state in ("supports", "contradicts", "unresolved")) != 2:
    raise SystemExit("relation formal-human comparison deleted or duplicated a terminal state")
if any(sum(row[state] for state in ("supports", "contradicts", "unresolved")) != row["denominator"] for row in formal_human["task_group_denominators"]):
    raise SystemExit("relation formal-human task-group denominators do not retain every state")
if len(resolutions) != 2 or {resolution["digest"] for resolution in resolutions} != set(formal_human["resolution_digests"]):
    raise SystemExit("relation formal-human comparison does not bind exact resolutions")
if any(resolution["verifier_relation_status"] != "not_consulted" or resolution["translation"]["state"] not in {"supports", "contradicts", "unresolved"} for resolution in resolutions):
    raise SystemExit("relation resolution collapsed the human and verifier evidence layers")
if terminal_ledger["formal_human_comparison_digest"] != formal_human["digest"] or len(terminal_ledger["entries"]) != 2:
    raise SystemExit("relation terminal ledger lost comparison or packet coverage")
if terminal_ledger["packet_states"] != formal_human["packet_states"] or any(entry["verifier_relation_status"] != "not_consulted" for entry in terminal_ledger["entries"]):
    raise SystemExit("relation terminal ledger changed states or consulted verifier evidence")

print(
    "Relation construct passed: "
    f"plan={plan['digest']} pilot={pilot['digest']} sample={sample['digest']} cases={sample['selected_cases']} "
    f"sources={sample['unique_source_ids']} task_groups={sample['unique_task_groups']} "
    f"units={sample['trajectory_pair_units']}pair+{sample['candidate_order_units']}pair-of-pairs "
    f"families={len(plan['families'])} axes={len(plan['axes'])} schemas={schema_count} replays=2 materials=2+8readiness packets=2+8readiness mappings=2+8readiness readiness=structural_pending launch_dossier=not_launchable inspection_workbook=owner_only inspection_fixture=synthetic_pass_not_authorized pilot_package=immutable_verified qualification=8 reviewers=3synthetic kits=3 primary_batches=2 probes=4 disagreements=1 tie_assignments=1 reveal=sealed comparison=sealed resolutions=2 ledger={terminal_ledger['status']} deep_bindings={len(sample['bindings'])} "
    f"zero_defect_upper={amendment['inference']['zero_contradiction_upper_bound']:.4f} empirical=not_run "
    "objective_isolation=mutual external_action_status=not_authorized providers=not_invoked"
)
PY
