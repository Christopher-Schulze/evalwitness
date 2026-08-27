#!/usr/bin/env bash
# Provider-free controlled-corruption corpus, schema, lineage, and control gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d)
tmp_bin=$(mktemp /tmp/evalwitness-corruption-XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" mutation corpus spec --spec @eval/governance/controlled-corruption-v1.json > "$tmp_dir/spec.json"
for schema in manifest witness blind-review-packet corpus-spec corpus-release reduction-witness formal-control; do
  "$tmp_bin" mutation schema --type "$schema" > "$tmp_dir/$schema.schema.json"
done
"$tmp_bin" mutation control validate \
  --original @eval/fixtures/controlled-corruption/original.json \
  --mutated @eval/fixtures/controlled-corruption/mutated.json > "$tmp_dir/control.json"

terminal_root="eval/trajectories/terminal_trajs/forge_gpt54"
swe_root="eval/trajectories/swebench_verified_trajs"
if [ ! -d "$terminal_root" ] || [ ! -d "$swe_root" ]; then
  echo "Controlled corruption core passed; full 320-case corpus skipped because fetched eval trajectories are absent."
  exit 0
fi

"$tmp_bin" mutation corpus build --root . --spec @eval/governance/controlled-corruption-v1.json > "$tmp_dir/release.json"
"$tmp_bin" mutation corpus validate --release @"$tmp_dir/release.json" > "$tmp_dir/validation.json"

python3 - "$tmp_dir" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])

def load(name):
    with (root / name).open(encoding="utf-8") as handle:
        return json.load(handle)

spec = load("spec.json")
control = load("control.json")
release = load("release.json")
validation = load("validation.json")

expected_digest = "6f60fc2ceac52fa9efbf4f5b39dc132ae62e849987cb08b5753f09941a8b020a"
if spec["mutators_frozen"] is not True or len(spec["primary_families"]) != 8:
    raise SystemExit("controlled-corruption specification is not frozen at eight primary families")
if control.get("valid") is not True or control["outcome_proof"]["original_passed"] is not True or control["outcome_proof"]["mutated_passed"] is not False:
    raise SystemExit("formal positive control did not reproduce a pass-to-fail outcome")
if validation.get("valid") is not True or validation.get("digest") != expected_digest:
    raise SystemExit("controlled-corruption release validation or governed digest changed")
if len(release["sources"]) != 200 or release["task_count"] != 100 or len(release["cases"]) != 320:
    raise SystemExit("controlled-corruption release size changed")
if len(release["source_family_counts"]) != 3 or any(row["count"] != 40 for row in release["mutation_family_counts"]):
    raise SystemExit("controlled-corruption source diversity or family balance changed")
if release["positive_controls"] != 1 or release["negative_controls"] != 120 or release["decoy_controls"] != 40 or release["ambiguous_cases"] != 0:
    raise SystemExit("controlled-corruption control composition changed")
if sum(case["manifest"]["review"]["required"] for case in release["cases"]) != 31:
    raise SystemExit("controlled-corruption blind-review sample changed")
if any(not case.get("blind_review_packet", {}).get("digest") for case in release["cases"]):
    raise SystemExit("controlled-corruption release contains an unsealed review packet")
task_splits = {}
lineage_fields = ("repository_id", "near_duplicate_id", "lineage_cluster_id", "split_group_id", "patch_digest")
lineage_splits = {field: {} for field in lineage_fields}
for source in release["sources"]:
    task_splits.setdefault(source["split"], set()).add(source["split_group_id"])
    for field in lineage_fields:
        value = source.get(field)
        if value:
            lineage_splits[field].setdefault(value, set()).add(source["split"])
if {role: len(groups) for role, groups in task_splits.items()} != {"development": 60, "calibration": 20, "test": 20}:
    raise SystemExit("controlled-corruption task split is not exactly 60/20/20")
if any(len(splits) != 1 for groups in lineage_splits.values() for splits in groups.values()):
    raise SystemExit("controlled-corruption repository, near-duplicate, task, patch, or lineage component crosses splits")

PY

python3 - "$tmp_dir/release.json" "$tmp_dir/tampered-release.json" <<'PY'
import json
import pathlib
import sys

source = pathlib.Path(sys.argv[1])
target = pathlib.Path(sys.argv[2])
with source.open(encoding="utf-8") as handle:
    release = json.load(handle)
release["cases"][0]["source_ids"] = release["cases"][1]["source_ids"]
with target.open("w", encoding="utf-8") as handle:
    json.dump(release, handle, separators=(",", ":"), ensure_ascii=False)
PY
if "$tmp_bin" mutation corpus validate --release @"$tmp_dir/tampered-release.json" >/dev/null 2>&1; then
  echo "controlled-corruption validation accepted tampered case lineage" >&2
  exit 1
fi
echo "Controlled corruption passed: digest=6f60fc2ceac52fa9efbf4f5b39dc132ae62e849987cb08b5753f09941a8b020a sources=200 tasks=100 task_split=60/20/20 cases=320 families=8 blind_review_sample=31 lineage_cross_split=0 tamper_rejected=true providers=not_invoked"
