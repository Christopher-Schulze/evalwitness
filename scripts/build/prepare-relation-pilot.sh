#!/usr/bin/env bash
# Prepare the complete owner-only TASK 068 relation pilot inspection package
# without contacting reviewers or providers. The destination must not exist.

set -euo pipefail
umask 077

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)

binary=""
packet_key_file=""
packet_key_id=""
qualification_key_file=""
qualification_key_id=""
package_root=""
bundle_created_at=""
readiness_prepared_at=""
dossier_prepared_at=""
governance_version="v1"
expected_pilot_cases=8

usage() {
  cat >&2 <<'EOF'
usage: scripts/build/prepare-relation-pilot.sh \
  --packet-key-file PATH --packet-key-id ID \
  --qualification-key-file PATH --qualification-key-id ID \
  --package-root PATH --bundle-created-at RFC3339 \
  --readiness-prepared-at RFC3339 --dossier-prepared-at RFC3339 \
  [--governance-version v1|v2|v3] [--binary PATH]

The command regenerates the governed corpus and samples, materializes every
governed pilot case, publishes private mappings and the qualification answer key,
seals the restricted bundle and readiness record, creates the content-addressed
change receipt, renders the receipt-bound owner-only all-difference atlas and
inspection workbook plus the public-safe launch brief, and verifies the
immutable package. Governance v1 is the historical default; v2 emits the
construct-audited historical package chain; v3 emits the typed natural-corpus
chain, binds the separately typed scarcity sentinel plus the public
falsification and repair evidence, and materializes a separate owner-only
scarcity-inspection appendix without adding pilot packets or labels. It never
generates, prints, sends, or replaces a key, package, inspection decision, or
human result.
EOF
}

while (($# > 0)); do
  case "$1" in
    --binary)
      binary=${2:-}
      shift 2
      ;;
    --packet-key-file)
      packet_key_file=${2:-}
      shift 2
      ;;
    --packet-key-id)
      packet_key_id=${2:-}
      shift 2
      ;;
    --qualification-key-file)
      qualification_key_file=${2:-}
      shift 2
      ;;
    --qualification-key-id)
      qualification_key_id=${2:-}
      shift 2
      ;;
    --package-root)
      package_root=${2:-}
      shift 2
      ;;
    --bundle-created-at)
      bundle_created_at=${2:-}
      shift 2
      ;;
    --readiness-prepared-at)
      readiness_prepared_at=${2:-}
      shift 2
      ;;
    --dossier-prepared-at)
      dossier_prepared_at=${2:-}
      shift 2
      ;;
    --governance-version)
      governance_version=${2:-}
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$packet_key_file" || -z "$packet_key_id" || -z "$qualification_key_file" || -z "$qualification_key_id" || -z "$package_root" || -z "$bundle_created_at" || -z "$readiness_prepared_at" || -z "$dossier_prepared_at" ]]; then
  echo "error: both key identities, package root, and all ordered timestamps are required" >&2
  usage
  exit 2
fi
if [[ "$governance_version" != v1 && "$governance_version" != v2 && "$governance_version" != v3 ]]; then
  echo "error: --governance-version must be v1, v2, or v3" >&2
  exit 2
fi
if [[ "$packet_key_id" == "$qualification_key_id" ]]; then
  echo "error: packet and qualification key identities must be distinct" >&2
  exit 1
fi

cd "$repo_root"

corpus_spec_path="eval/governance/controlled-corruption-$governance_version.json"
corpus_plan_path=""
corpus_audit_path=""
plan_path="eval/governance/relation-audit-plan-$governance_version.json"
primary_path="eval/governance/relation-primary-sample-$governance_version.json"
pilot_path="eval/governance/relation-pilot-sample-$governance_version.json"
amendment_path="eval/governance/relation-study-amendment-$governance_version.json"
sentinel_path=""
schema_suffix=""
package_format="evalwitness.relation-pilot-package.v2"
if [[ "$governance_version" == v2 ]]; then
  schema_suffix="-v2"
  package_format="evalwitness.relation-pilot-package.v3"
elif [[ "$governance_version" == v3 ]]; then
  corpus_spec_path=""
  corpus_plan_path="eval/governance/controlled-corruption-v3-plan.json"
  corpus_audit_path="eval/governance/controlled-corruption-v3-natural-audit.json"
  sentinel_path="eval/governance/relation-scarcity-sentinel-v3.json"
  schema_suffix="-v3"
  package_format="evalwitness.relation-pilot-package.v5"
  expected_pilot_cases=7
fi

for key_file in "$packet_key_file" "$qualification_key_file"; do
  if [[ ! -f "$key_file" || -L "$key_file" ]]; then
    echo "error: key inputs must be existing regular non-symlink files" >&2
    exit 1
  fi
  key_mode=$(stat -f '%Lp' "$key_file" 2>/dev/null || stat -c '%a' "$key_file")
  if [[ "$key_mode" != 600 ]]; then
    echo "error: key inputs must be mode 0600" >&2
    exit 1
  fi
done
if cmp -s "$packet_key_file" "$qualification_key_file"; then
  echo "error: packet and qualification keys must contain distinct key material" >&2
  exit 1
fi
if [[ -e "$package_root" || -L "$package_root" ]]; then
  echo "error: --package-root already exists; prepared pilot packages are immutable" >&2
  exit 1
fi
if [[ ! -d eval/trajectories/terminal_trajs/forge_gpt54 || ! -d eval/trajectories/swebench_verified_trajs ]]; then
  echo "error: fetched Terminal-Bench and SWE-bench trajectory assets are required" >&2
  exit 1
fi

package_parent=$(dirname "$package_root")
package_name=$(basename "$package_root")
if [[ "$package_name" == "." || "$package_name" == ".." || "$package_name" == "/" || -z "$package_name" ]]; then
  echo "error: --package-root must name a new specific directory" >&2
  exit 1
fi
mkdir -p "$package_parent"

staging_root=$(mktemp -d "$package_parent/.${package_name}.candidate.XXXXXXXX")
work_dir=$(mktemp -d /tmp/evalwitness-relation-pilot-prepare-XXXXXXXX)
published=false

cleanup() {
  if [[ "$published" != true && -n "$staging_root" && -d "$staging_root" ]]; then
    rm -rf "$staging_root"
  fi
  if [[ -n "$work_dir" && -d "$work_dir" ]]; then
    rm -rf "$work_dir"
  fi
}
trap cleanup EXIT

if [[ -z "$binary" ]]; then
  binary="$work_dir/evalwitness"
  go build -o "$binary" ./cmd/evalwitness
elif [[ ! -f "$binary" || -L "$binary" || ! -x "$binary" ]]; then
  echo "error: --binary must name an executable regular non-symlink file" >&2
  exit 1
fi

if [[ "$governance_version" == v3 ]]; then
  audit_date=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["audited_at"])' "$corpus_audit_path")
  "$binary" mutation corpus plan-v3 > "$work_dir/controlled-corruption-plan.json"
  cmp "$work_dir/controlled-corruption-plan.json" "$corpus_plan_path" > /dev/null
  "$binary" mutation corpus audit-v3 \
    --root . --plan @"$work_dir/controlled-corruption-plan.json" \
    --audited-at "$audit_date" > "$work_dir/controlled-corruption-audit.json"
  cmp "$work_dir/controlled-corruption-audit.json" "$corpus_audit_path" > /dev/null
  "$binary" mutation corpus build-v3 \
    --root . --plan @"$work_dir/controlled-corruption-plan.json" \
    --audit @"$work_dir/controlled-corruption-audit.json" \
    > "$work_dir/controlled-corruption-release.json"
  "$binary" mutation corpus validate-v3-release \
    --plan @"$work_dir/controlled-corruption-plan.json" \
    --audit @"$work_dir/controlled-corruption-audit.json" \
    --release @"$work_dir/controlled-corruption-release.json" > /dev/null
  cmp "$work_dir/controlled-corruption-release.json" eval/governance/controlled-corruption-v3-release.json > /dev/null
  "$binary" relation primary-sample-v3 \
    --plan "@$plan_path" \
    --corpus-plan @"$work_dir/controlled-corruption-plan.json" \
    --corpus-audit @"$work_dir/controlled-corruption-audit.json" \
    --release @"$work_dir/controlled-corruption-release.json" \
    > "$work_dir/relation-primary-sample.json"
  cmp "$work_dir/relation-primary-sample.json" "$primary_path" > /dev/null
  "$binary" relation scarcity-sentinel-v3 \
    --plan "@$plan_path" \
    --primary-sample @"$work_dir/relation-primary-sample.json" \
    --corpus-plan @"$work_dir/controlled-corruption-plan.json" \
    --corpus-audit @"$work_dir/controlled-corruption-audit.json" \
    --release @"$work_dir/controlled-corruption-release.json" \
    > "$work_dir/relation-scarcity-sentinel.json"
  cmp "$work_dir/relation-scarcity-sentinel.json" "$sentinel_path" > /dev/null
  "$binary" relation pilot-sample-v3 \
    --plan "@$plan_path" \
    --primary-sample @"$work_dir/relation-primary-sample.json" \
    --scarcity-sentinel @"$work_dir/relation-scarcity-sentinel.json" \
    --corpus-plan @"$work_dir/controlled-corruption-plan.json" \
    --corpus-audit @"$work_dir/controlled-corruption-audit.json" \
    --release @"$work_dir/controlled-corruption-release.json" \
    > "$work_dir/relation-pilot-sample.json"
  cmp "$work_dir/relation-pilot-sample.json" "$pilot_path" > /dev/null
  amendment_issued_at=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["issued_at"])' "$amendment_path")
  "$binary" relation study-amendment-v3 \
    --plan "@$plan_path" \
    --primary-sample @"$work_dir/relation-primary-sample.json" \
    --scarcity-sentinel @"$work_dir/relation-scarcity-sentinel.json" \
    --pilot-sample @"$work_dir/relation-pilot-sample.json" \
    --issued-at "$amendment_issued_at" > "$work_dir/relation-study-amendment.json"
  cmp "$work_dir/relation-study-amendment.json" "$amendment_path" > /dev/null
  "$binary" mutation construct-challenge build > "$work_dir/construct-firewall-challenge.json"
  cmp "$work_dir/construct-firewall-challenge.json" eval/governance/construct-firewall-challenge-v1.json > /dev/null
  "$binary" mutation construct-challenge validate \
    --evidence @"$work_dir/construct-firewall-challenge.json" > /dev/null
  cp eval/governance/construct-repair-evidence-v1.json "$work_dir/construct-repair-evidence.json"
  "$binary" artifact scan --class public \
    --path "$work_dir/construct-firewall-challenge.json" > /dev/null
  "$binary" artifact scan --class public \
    --path "$work_dir/construct-repair-evidence.json" > /dev/null
else
  "$binary" mutation corpus build \
    --root . --spec "@$corpus_spec_path" \
    > "$work_dir/controlled-corruption-release.json"
  "$binary" mutation corpus validate \
    --release @"$work_dir/controlled-corruption-release.json" > /dev/null
  "$binary" relation primary-sample \
    --plan "@$plan_path" \
    --release @"$work_dir/controlled-corruption-release.json" \
    > "$work_dir/relation-primary-sample.json"
  cmp "$work_dir/relation-primary-sample.json" "$primary_path" > /dev/null
  "$binary" relation pilot-sample \
    --plan "@$plan_path" \
    --primary-sample @"$work_dir/relation-primary-sample.json" \
    --release @"$work_dir/controlled-corruption-release.json" \
    > "$work_dir/relation-pilot-sample.json"
  cmp "$work_dir/relation-pilot-sample.json" "$pilot_path" > /dev/null
  amendment_issued_at=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["issued_at"])' "$amendment_path")
  "$binary" relation study-amendment \
    --plan "@$plan_path" \
    --pilot-sample @"$work_dir/relation-pilot-sample.json" \
    --primary-sample @"$work_dir/relation-primary-sample.json" \
    --issued-at "$amendment_issued_at" > "$work_dir/relation-study-amendment.json"
  cmp "$work_dir/relation-study-amendment.json" "$amendment_path" > /dev/null
fi

# Initialize the protected package root before placing any other artifact in it.
"$binary" relation qualification \
  --plan "@$plan_path" \
  --key-file "$qualification_key_file" --key-id "$qualification_key_id" \
  --private-root "$staging_root" > "$work_dir/qualification-set.json"
"$binary" relation validate --type qualification-set \
  --document @"$work_dir/qualification-set.json" > /dev/null
qualification_key_files=("$staging_root"/qualification-keys/*.json)
if [[ ${#qualification_key_files[@]} -ne 1 ]]; then
  echo "error: qualification did not publish exactly one answer key" >&2
  exit 1
fi
"$binary" relation validate --type qualification-answer-key \
  --document @"${qualification_key_files[0]}" > /dev/null
"$binary" relation handbook \
  --plan "@$plan_path" \
  --qualification-set @"$work_dir/qualification-set.json" \
  > "$work_dir/reviewer-handbook.json"
"$binary" relation validate --type reviewer-handbook \
  --document @"$work_dir/reviewer-handbook.json" > /dev/null

mkdir -p "$staging_root/materials" "$staging_root/packets"
case_index=0
while IFS= read -r case_id; do
  case_index=$((case_index + 1))
  printf -v ordinal '%02d' "$case_index"
  material_path="$staging_root/materials/$ordinal.json"
  packet_path="$staging_root/packets/$ordinal.json"
  if [[ "$governance_version" == v3 ]]; then
    "$binary" relation materialize-v3 \
      --root . --plan "@$plan_path" \
      --corpus-plan @"$work_dir/controlled-corruption-plan.json" \
      --corpus-audit @"$work_dir/controlled-corruption-audit.json" \
      --release @"$work_dir/controlled-corruption-release.json" \
      --case-id "$case_id" > "$material_path"
  else
    "$binary" relation materialize \
      --root . --plan "@$plan_path" \
      --release @"$work_dir/controlled-corruption-release.json" \
      --case-id "$case_id" > "$material_path"
  fi
  "$binary" relation validate --type case-material --document @"$material_path" > /dev/null
  if [[ "$governance_version" == v3 ]]; then
    "$binary" relation packet-v3 \
      --plan "@$plan_path" \
      --corpus-plan @"$work_dir/controlled-corruption-plan.json" \
      --corpus-audit @"$work_dir/controlled-corruption-audit.json" \
      --release @"$work_dir/controlled-corruption-release.json" \
      --material @"$material_path" --key-file "$packet_key_file" \
      --key-id "$packet_key_id" --private-root "$staging_root" > "$packet_path"
  else
    "$binary" relation packet \
      --plan "@$plan_path" \
      --release @"$work_dir/controlled-corruption-release.json" \
      --material @"$material_path" --key-file "$packet_key_file" \
      --key-id "$packet_key_id" --private-root "$staging_root" > "$packet_path"
  fi
  "$binary" relation validate --type blind-packet --document @"$packet_path" > /dev/null
done < <(python3 -c 'import json,sys; print("\n".join(case["case_id"] for case in json.load(open(sys.argv[1], encoding="utf-8"))["cases"]))' "$pilot_path")
if [[ "$case_index" -ne "$expected_pilot_cases" ]]; then
  echo "error: relation pilot preparation did not materialize all $expected_pilot_cases governed cases" >&2
  exit 1
fi

scarcity_material_args=()
if [[ "$governance_version" == v3 ]]; then
  mkdir -p "$staging_root/sentinel-materials"
  sentinel_index=0
  while IFS= read -r case_id; do
    sentinel_index=$((sentinel_index + 1))
    printf -v ordinal '%02d' "$sentinel_index"
    material_path="$staging_root/sentinel-materials/$ordinal.json"
    "$binary" relation materialize-v3 \
      --root . --plan "@$plan_path" \
      --corpus-plan @"$work_dir/controlled-corruption-plan.json" \
      --corpus-audit @"$work_dir/controlled-corruption-audit.json" \
      --release @"$work_dir/controlled-corruption-release.json" \
      --case-id "$case_id" > "$material_path"
    "$binary" relation validate --type case-material --document @"$material_path" > /dev/null
    scarcity_material_args+=(--material @"$material_path")
  done < <(python3 -c 'import json,sys; print("\n".join(case["case_id"] for case in json.load(open(sys.argv[1], encoding="utf-8"))["cases"]))' "$sentinel_path")
  if [[ "$sentinel_index" -ne 3 ]]; then
    echo "error: relation pilot preparation did not materialize all three exhaustive scarcity-sentinel cases" >&2
    exit 1
  fi
fi

python3 - "$staging_root" "$expected_pilot_cases" <<'PY'
import json
import os
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
expected_cases = int(sys.argv[2])
packets = [json.loads(path.read_text(encoding="utf-8")) for path in sorted((root / "packets").glob("*.json"))]
mappings = [json.loads(path.read_text(encoding="utf-8")) for path in sorted((root / "mappings").glob("*.json"))]
if len(packets) != expected_cases or len(mappings) != expected_cases:
    raise SystemExit(f"relation pilot package requires exactly {expected_cases} packets and mappings")
mappings.sort(key=lambda value: value["packet_id"])
(root / "blind-packets.json").write_text(json.dumps(packets, indent=2) + "\n", encoding="utf-8")
(root / "private-mappings.json").write_text(json.dumps(mappings, indent=2) + "\n", encoding="utf-8")
os.chmod(root / "blind-packets.json", 0o600)
os.chmod(root / "private-mappings.json", 0o600)
PY

cp "$work_dir/controlled-corruption-release.json" "$staging_root/controlled-corruption-release.json"
cp "$plan_path" "$staging_root/relation-audit-plan.json"
cp "$work_dir/relation-primary-sample.json" "$staging_root/relation-primary-sample.json"
cp "$work_dir/relation-pilot-sample.json" "$staging_root/relation-pilot-sample.json"
cp "$work_dir/relation-study-amendment.json" "$staging_root/relation-study-amendment.json"
cp "$work_dir/qualification-set.json" "$staging_root/qualification-set.json"
cp "$work_dir/reviewer-handbook.json" "$staging_root/reviewer-handbook.json"
pilot_v3_args=()
if [[ "$governance_version" == v3 ]]; then
  cp "$work_dir/controlled-corruption-plan.json" "$staging_root/controlled-corruption-plan.json"
  cp "$work_dir/controlled-corruption-audit.json" "$staging_root/controlled-corruption-audit.json"
  cp "$work_dir/relation-scarcity-sentinel.json" "$staging_root/relation-scarcity-sentinel.json"
  cp "$work_dir/construct-firewall-challenge.json" "$staging_root/construct-firewall-challenge.json"
  cp "$work_dir/construct-repair-evidence.json" "$staging_root/construct-repair-evidence.json"
  pilot_v3_args=(
    --primary-sample-v3 @"$staging_root/relation-primary-sample.json"
    --scarcity-sentinel-v3 @"$staging_root/relation-scarcity-sentinel.json"
  )
fi

pilot_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["digest"])' "$pilot_path")
"$binary" relation bundle \
  --plan @"$staging_root/relation-audit-plan.json" \
  --sample-digest "$pilot_digest" --data-role development_pilot \
  --packets @"$staging_root/blind-packets.json" \
  --mappings @"$staging_root/private-mappings.json" \
  --qualification-set @"$staging_root/qualification-set.json" \
  --handbook @"$staging_root/reviewer-handbook.json" \
  --created-at "$bundle_created_at" > "$staging_root/review-bundle.json"
"$binary" relation pilot-readiness \
  --plan @"$staging_root/relation-audit-plan.json" \
  --pilot-sample @"$staging_root/relation-pilot-sample.json" \
  "${pilot_v3_args[@]}" \
  --bundle @"$staging_root/review-bundle.json" \
  --mappings @"$staging_root/private-mappings.json" \
  --qualification-set @"$staging_root/qualification-set.json" \
  --handbook @"$staging_root/reviewer-handbook.json" \
  --prepared-at "$readiness_prepared_at" > "$staging_root/pilot-readiness.json"
"$binary" relation pilot-change-receipt \
  --readiness @"$staging_root/pilot-readiness.json" \
  --bundle @"$staging_root/review-bundle.json" \
  --mappings @"$staging_root/private-mappings.json" \
  > "$staging_root/pilot-change-receipt.json"
"$binary" relation validate --type pilot-change-receipt \
  --document @"$staging_root/pilot-change-receipt.json" > /dev/null
"$binary" relation pilot-launch-dossier \
  --plan @"$staging_root/relation-audit-plan.json" \
  --pilot-sample @"$staging_root/relation-pilot-sample.json" \
  "${pilot_v3_args[@]}" \
  --bundle @"$staging_root/review-bundle.json" \
  --mappings @"$staging_root/private-mappings.json" \
  --qualification-set @"$staging_root/qualification-set.json" \
  --handbook @"$staging_root/reviewer-handbook.json" \
  --readiness @"$staging_root/pilot-readiness.json" \
  --prepared-at "$dossier_prepared_at" > "$staging_root/pilot-launch-dossier.json"
"$binary" relation render-pilot-launch-brief \
  --dossier @"$staging_root/pilot-launch-dossier.json" \
  > "$staging_root/pilot-launch-brief.md"
"$binary" artifact scan --class public \
  --path "$staging_root/pilot-launch-brief.md" > /dev/null
"$binary" relation render-pilot-change-atlas \
  --receipt @"$staging_root/pilot-change-receipt.json" \
  --readiness @"$staging_root/pilot-readiness.json" \
  --bundle @"$staging_root/review-bundle.json" \
  --mappings @"$staging_root/private-mappings.json" \
  > "$staging_root/owner-change-atlas.md"
"$binary" relation render-pilot-inspection \
  --readiness @"$staging_root/pilot-readiness.json" \
  --bundle @"$staging_root/review-bundle.json" \
  --mappings @"$staging_root/private-mappings.json" \
  --handbook @"$staging_root/reviewer-handbook.json" \
  > "$staging_root/owner-inspection.md"
if [[ "$governance_version" == v3 ]]; then
  "$binary" relation render-scarcity-inspection \
    --root . \
    --plan @"$staging_root/relation-audit-plan.json" \
    --primary-sample @"$staging_root/relation-primary-sample.json" \
    --scarcity-sentinel @"$staging_root/relation-scarcity-sentinel.json" \
    --corpus-plan @"$staging_root/controlled-corruption-plan.json" \
    --corpus-audit @"$staging_root/controlled-corruption-audit.json" \
    --release @"$staging_root/controlled-corruption-release.json" \
    "${scarcity_material_args[@]}" \
    > "$staging_root/owner-scarcity-inspection.md"
fi
"$binary" relation schema --type "pilot-inspection$schema_suffix" > "$staging_root/pilot-inspection.schema.json"
"$binary" relation schema --type "pilot-launch-dossier$schema_suffix" > "$staging_root/pilot-launch-dossier.schema.json"
"$binary" relation schema --type "pilot-change-receipt$schema_suffix" > "$staging_root/pilot-change-receipt.schema.json"

python3 - "$staging_root" "$bundle_created_at" "$readiness_prepared_at" "$dossier_prepared_at" "$package_format" "$governance_version" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
release = json.loads((root / "controlled-corruption-release.json").read_text(encoding="utf-8"))
primary = json.loads((root / "relation-primary-sample.json").read_text(encoding="utf-8"))
pilot = json.loads((root / "relation-pilot-sample.json").read_text(encoding="utf-8"))
amendment = json.loads((root / "relation-study-amendment.json").read_text(encoding="utf-8"))
qualification = json.loads((root / "qualification-set.json").read_text(encoding="utf-8"))
handbook = json.loads((root / "reviewer-handbook.json").read_text(encoding="utf-8"))
bundle = json.loads((root / "review-bundle.json").read_text(encoding="utf-8"))
readiness = json.loads((root / "pilot-readiness.json").read_text(encoding="utf-8"))
dossier = json.loads((root / "pilot-launch-dossier.json").read_text(encoding="utf-8"))
change_receipt = json.loads((root / "pilot-change-receipt.json").read_text(encoding="utf-8"))
summary = {
    "format_version": sys.argv[5],
    "review_objective": "controlled_relation",
    "source_corpus_digest": release["digest"],
    "primary_sample_digest": primary["digest"],
    "pilot_sample_digest": pilot["digest"],
    "study_amendment_digest": amendment["digest"],
    "qualification_set_digest": qualification["digest"],
    "handbook_digest": handbook["digest"],
    "bundle_digest": bundle["digest"],
    "readiness_digest": readiness["digest"],
    "launch_dossier_digest": dossier["digest"],
    "launch_brief_policy": "evalwitness.relation-pilot-launch-brief.v1",
    "launch_brief_public_safety_status": "passed",
    "change_atlas_policy": "evalwitness.relation-pilot-change-atlas.v1",
    "change_atlas_status": "generated_owner_only_receipt_bound_no_decisions",
    "change_receipt_schema_version": change_receipt["schema_version"],
    "change_receipt_policy": change_receipt["receipt_policy"],
    "change_receipt_digest": change_receipt["digest"],
    "change_receipt_status": "verified_owner_only_no_raw_content_no_decisions",
    "mapping_commitment_digest": readiness["mapping_commitment_digest"],
    "packets": readiness["packets"],
    "families": sorted(check["family"] for check in readiness["packet_checks"]),
    "bundle_created_at": sys.argv[2],
    "readiness_prepared_at": sys.argv[3],
    "dossier_prepared_at": sys.argv[4],
    "technical_status": readiness["technical_status"],
    "semantic_inspection_status": readiness["semantic_inspection_status"],
    "owner_inspection_status": "not_completed",
    "launch_status": dossier["launch_status"],
    "human_study_status": "not_run",
    "empirical_status": "not_run",
    "external_action_status": "not_authorized",
    "required_next_action": readiness["required_external_action"],
    "limitations": [
        "Package checksums provide integrity, not publisher authenticity.",
        "The package contains restricted reference-only evidence and hidden mappings.",
        "No owner semantic decision, independent reviewer result, or provider result is included.",
    ],
}
if sys.argv[6] == "v2":
    summary.update({
        "relation_protocol_version": bundle["protocol_version"],
        "source_corpus_version": release["corpus_version"],
        "source_corpus_spec_digest": release["spec_digest"],
        "source_mutation_program_digest": release["mutation_program_digest"],
        "source_construct_audit_digest": release["spec"]["development_audit"]["construct_audit_digest"],
        "construct_firewall_commitment_digest": readiness["construct_firewall_commitment_digest"],
        "plan_schema_version": json.loads((root / "relation-audit-plan.json").read_text(encoding="utf-8"))["schema_version"],
        "primary_sample_schema_version": primary["schema_version"],
        "pilot_sample_schema_version": pilot["schema_version"],
        "study_amendment_schema_version": amendment["schema_version"],
        "qualification_set_schema_version": qualification["schema_version"],
        "handbook_schema_version": handbook["schema_version"],
        "bundle_schema_version": bundle["schema_version"],
        "readiness_schema_version": readiness["schema_version"],
        "launch_dossier_schema_version": dossier["schema_version"],
        "primary_cases": primary["selected_cases"],
        "primary_task_groups": primary["unique_task_groups"],
        "primary_lineage_clusters": primary["unique_lineage_clusters"],
        "pilot_task_groups": pilot["unique_task_groups"],
        "pilot_lineage_clusters": pilot["unique_lineage_clusters"],
        "pilot_primary_overlap": pilot["primary_overlap"],
    })
elif sys.argv[6] == "v3":
    corpus_plan = json.loads((root / "controlled-corruption-plan.json").read_text(encoding="utf-8"))
    corpus_audit = json.loads((root / "controlled-corruption-audit.json").read_text(encoding="utf-8"))
    sentinel = json.loads((root / "relation-scarcity-sentinel.json").read_text(encoding="utf-8"))
    challenge = json.loads((root / "construct-firewall-challenge.json").read_text(encoding="utf-8"))
    repair = json.loads((root / "construct-repair-evidence.json").read_text(encoding="utf-8"))
    plan = json.loads((root / "relation-audit-plan.json").read_text(encoding="utf-8"))
    summary.update({
        "relation_protocol_version": bundle["protocol_version"],
        "source_corpus_version": release["corpus_version"],
        "source_corpus_plan_digest": corpus_plan["digest"],
        "source_corpus_audit_digest": corpus_audit["digest"],
        "source_mutation_program_digest": release["mutation_program_digest"],
        "source_construct_audit_digest": plan["source_construct_audit_digest"],
        "construct_firewall_commitment_digest": readiness["construct_firewall_commitment_digest"],
        "scarcity_sentinel_digest": sentinel["digest"],
        "scarcity_sentinel_cases": sentinel["selected_cases"],
        "scarcity_sentinel_primary_overlap": sentinel["primary_overlap"],
        "scarcity_sentinel_pilot_overlap": pilot["scarcity_sentinel_overlap"],
        "sentinel_in_primary_estimand": plan["sentinel_in_primary_estimand"],
        "held_out_sentinel_claim_available": sentinel["held_out_claim_available"],
        "scarcity_inspection_policy": "evalwitness.relation-scarcity-owner-inspection.v1",
        "scarcity_inspection_status": "generated_owner_only_no_decisions",
        "scarcity_materials": sentinel["selected_cases"],
        "scarcity_materialization_status": "complete_replay_bound_restricted",
        "construct_challenge_digest": challenge["digest"],
        "construct_repair_evidence_digest": repair["digest"],
        "construct_evidence_status": "public_safe_frozen_no_human_provider_or_population_claim",
        "inventory_policy": "evalwitness.relation-pilot-package-inventory.v1",
        "empirical_state_inheritance": "none",
        "plan_schema_version": plan["schema_version"],
        "corpus_plan_schema_version": corpus_plan["schema_version"],
        "corpus_audit_schema_version": corpus_audit["schema_version"],
        "corpus_release_schema_version": release["schema_version"],
        "primary_sample_schema_version": primary["schema_version"],
        "scarcity_sentinel_schema_version": sentinel["schema_version"],
        "pilot_sample_schema_version": pilot["schema_version"],
        "study_amendment_schema_version": amendment["schema_version"],
        "qualification_set_schema_version": qualification["schema_version"],
        "handbook_schema_version": handbook["schema_version"],
        "bundle_schema_version": bundle["schema_version"],
        "readiness_schema_version": readiness["schema_version"],
        "launch_dossier_schema_version": dossier["schema_version"],
        "primary_cases": primary["selected_cases"],
        "primary_task_groups": primary["unique_task_groups"],
        "primary_lineage_clusters": primary["unique_lineage_clusters"],
        "pilot_task_groups": pilot["unique_task_groups"],
        "pilot_lineage_clusters": pilot["unique_lineage_clusters"],
        "pilot_primary_overlap": pilot["primary_overlap"],
    })
(root / "package-summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
PY

cat > "$staging_root/OPERATING-INSTRUCTIONS.md" <<'EOF'
# EvalWitness Relation Pilot Owner Inspection

This is an owner-only, restricted, immutable input package. It contains the
exact governed development pilot, hidden mappings, reviewer-visible packets,
qualification custody, structural readiness, and the deterministic inspection
workbook. `pilot-change-receipt.json` is the content-addressed, schema-validated
machine evidence behind the atlas. It contains parent, content, changed-line,
lineage, denominator, reversal, and hazard bindings but no raw task or trajectory
content and no decision. `owner-change-atlas.md` binds that receipt while showing
the complete task, hidden alignment, and every non-common rendered line for all
governed pairs. It verifies exact candidate reversal and flags structural questions,
but never makes a semantic decision and never replaces the complete workbook.
`pilot-launch-dossier.json`
gives the exact maximum review workload,
structural packet disclosures, unresolved governance decisions, and explicitly
unauthorized external actions. `pilot-launch-brief.md` is the independently
reproducible public-safe decision surface; it contains protocol structure and
proposed defaults, never raw evidence, packet identity, mappings, human results,
or authorization. The package contains no completed owner decision and
authorizes no reviewer, provider, publication, or distribution action.

Package format v5 additionally carries the typed v3 natural-corpus plan, audit,
scarcity-sentinel commitment, public challenge/repair evidence, three restricted
replay-bound sentinel materials, and `owner-scarcity-inspection.md`. The sentinel
appendix is descriptive and remains outside the seven-family pilot bundle and
primary estimand. It exposes the exact negative-evidence construct surface and
its 2-development, 1-calibration, 0-test scarcity without adding a packet, label,
held-out claim, human result, provider result, or inherited empirical state.

Use `owner-change-atlas.md` to locate and understand every rendered difference,
then inspect the complete context in `owner-inspection.md` packet by packet.
Inspect `owner-scarcity-inspection.md` separately to document construct
availability and ambiguity without converting its prompts into pilot judgments.
Record exactly one assessment for every listed dimension and use only the
canonical reason codes shown in the workbook. Store the resulting decision
array as a separate mode-0600 file. Seal it with `evalwitness relation
pilot-inspection`, using this package's readiness, bundle, and private mappings
plus a separate private root. Then run
`evalwitness relation verify-pilot-inspection` against the sealed record and this
package. Any revision-required or unresolved packet blocks reviewer
qualification until a newly versioned packet, rubric, translation, or package
resolves it.

Run `scripts/audits/verify-relation-pilot-package.sh --package-root PATH` from a
matching EvalWitness checkout before inspection. `SHA256SUMS` detects accidental
or unauthorized byte changes but is not a signature.
EOF

find "$staging_root" -type d -exec chmod 0700 {} +
find "$staging_root" -type f -exec chmod 0600 {} +
if [[ "$governance_version" == v3 ]]; then
  python3 - "$staging_root" "$package_format" <<'PY'
import hashlib
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
directories = [
    {"path": str(path.relative_to(root)), "mode": "0700"}
    for path in sorted(candidate for candidate in root.rglob("*") if candidate.is_dir())
]
files = []
for path in sorted(candidate for candidate in root.rglob("*") if candidate.is_file() and candidate.name not in {"package-inventory.json", "SHA256SUMS"}):
    content = path.read_bytes()
    files.append({
        "path": str(path.relative_to(root)),
        "bytes": len(content),
        "mode": "0600",
        "sha256": hashlib.sha256(content).hexdigest(),
    })
body = {
    "schema_version": "evalwitness.relation-pilot-package-inventory.v1",
    "package_format": sys.argv[2],
    "hash_algorithm": "sha256",
    "scope": "all_package_payload_files_except_inventory_and_sha256sums",
    "directories": directories,
    "files": files,
    "payload_files": len(files),
    "payload_bytes": sum(item["bytes"] for item in files),
}
canonical = json.dumps(body, sort_keys=True, separators=(",", ":")).encode("utf-8")
body["digest"] = hashlib.sha256(canonical).hexdigest()
(root / "package-inventory.json").write_text(json.dumps(body, indent=2) + "\n", encoding="utf-8")
PY
  chmod 0600 "$staging_root/package-inventory.json"
fi
python3 - "$staging_root" <<'PY'
import hashlib
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
lines = []
for path in sorted(candidate for candidate in root.rglob("*") if candidate.is_file() and candidate.name != "SHA256SUMS"):
    lines.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.relative_to(root)}")
(root / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="ascii")
(root / "SHA256SUMS").chmod(0o600)
PY

scripts/audits/verify-relation-pilot-package.sh \
  --binary "$binary" --package-root "$staging_root" > /dev/null

if [[ -e "$package_root" || -L "$package_root" ]]; then
  echo "error: --package-root appeared during preparation; refusing to replace it" >&2
  exit 1
fi
mv "$staging_root" "$package_root"
published=true

summary_digest=$(shasum -a 256 "$package_root/package-summary.json" | awk '{print $1}')
echo "Relation pilot inspection package prepared without provider or reviewer action."
echo "Package root: $package_root"
echo "Pilot digest: $pilot_digest"
echo "Package summary digest: $summary_digest"
if [[ -f "$package_root/package-inventory.json" ]]; then
  inventory_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["digest"])' "$package_root/package-inventory.json")
  echo "Package inventory digest: $inventory_digest"
fi
echo "Owner inspection: not_completed; human_study_status=not_run; external_action_status=not_authorized"
