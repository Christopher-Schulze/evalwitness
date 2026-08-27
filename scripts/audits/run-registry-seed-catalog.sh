#!/usr/bin/env bash
# Provider-free development seed catalog gate for TASK 060.
#
# Validates the committed contrasting intake pair. This is a seed catalog,
# not a community registry, signed matrix, or TASK 050/058 admission.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
catalog="$repo_root/eval/governance/registry-seed-catalog-v1.json"

[[ -f $catalog ]] || { echo "missing seed catalog: $catalog" >&2; exit 2; }

tmp_bin=$(mktemp /tmp/evalwitness-registry-seed-bin-XXXXXXXX)
tmp_refresh=$(mktemp /tmp/evalwitness-registry-seed-refresh-XXXXXXXX)
tmp_matrix=$(mktemp /tmp/evalwitness-registry-seed-matrix-XXXXXXXX)
cleanup() { rm -f "$tmp_bin" "$tmp_refresh" "$tmp_matrix"; }
trap cleanup EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness
"$tmp_bin" registry refresh --catalog "$catalog" > "$tmp_refresh"
"$tmp_bin" registry render-matrix --catalog "$catalog" > "$tmp_matrix"

python3 - "$tmp_refresh" "$tmp_matrix" <<'PY'
import json
import sys

refresh = json.load(open(sys.argv[1]))
matrix = json.load(open(sys.argv[2]))
if refresh.get("rejected") != 0 or refresh.get("current") != 2:
    raise SystemExit(f"seed refresh not current: {refresh}")
if len(matrix.get("cells") or []) != 2:
    raise SystemExit(f"seed matrix cells != 2: {matrix}")
limitations = " ".join(matrix.get("limitations") or []).lower()
if "no ranking" not in limitations:
    raise SystemExit(f"seed matrix missing no-ranking limitation: {matrix}")
PY

echo "registry seed catalog: 2 contrasting format_verified development entries"
