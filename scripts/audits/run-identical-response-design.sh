#!/usr/bin/env bash
# Provider-free identical-response design preflight for TASK 070.
#
# Two decision arms read the same immutable completion: distribution-aware
# score evidence versus chosen-token extraction. This script reproduces the
# frozen paired design (detectable effect + failure sensitivity) from the
# committed spec and byte-compares it against the committed report.
#
# Usage: run-identical-response-design.sh
#
# Needs no provider, no network, and no API key. The design is an exact
# conditional-McNemar calculation; no Monte Carlo randomness is involved.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)

spec="$repo_root/eval/governance/identical-response-design-spec-v1.json"
report="$repo_root/eval/governance/identical-response-design-report-v1.json"

for path in "$spec" "$report"; do
    [[ -f $path ]] || { echo "missing frozen artifact: $path" >&2; exit 2; }
done

tmp_bin=$(mktemp /tmp/evalwitness-identical-response-bin-XXXXXXXX)
tmp_report=$(mktemp /tmp/evalwitness-identical-response-report-XXXXXXXX)
cleanup() { rm -f "$tmp_bin" "$tmp_report"; }
trap cleanup EXIT

go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" design identical-response --spec "$spec" --output json > "$tmp_report"

if ! diff -u "$report" "$tmp_report" > /dev/null; then
    echo "identical-response design report drifted from the committed artifact" >&2
    diff -u "$report" "$tmp_report" >&2 || true
    exit 1
fi

python3 - "$spec" "$report" <<'PY'
import hashlib
import json
import pathlib
import sys

spec_path, report_path = sys.argv[1], sys.argv[2]
spec = json.loads(pathlib.Path(spec_path).read_text(encoding="utf-8"))
report = json.loads(pathlib.Path(report_path).read_text(encoding="utf-8"))

assert report["counterfactual"] == "distribution_aware_vs_chosen_token"
assert report["algorithm"] == "evalwitness.identical-response-paired-design.v1"
assert report["source_task_groups"] == spec["source_task_groups"]
assert report["design_digest"]
assert len(report["disagreement_sensitivity"]) == len(spec["disagreement_rates"])
assert len(report["failure_sensitivity"]) == 5

# The committed spec must freeze the exact assumptions, and the report must
# record its own digest so a reproduced run is byte-checkable.
expected_loss = (
    spec["invalid_rate"] + spec["missing_rate"] + spec["abstention_rate"] + spec["route_failure_rate"]
)
assert abs(report["combined_loss_fraction"] - expected_loss) < 1e-9

# The exact conditional McNemar design never fabricates an MDE for a rate
# whose complete-separation power is below target: MDE rows must be absent
# exactly when power_at_complete_separation_nominal < target_power.
for row in report["disagreement_sensitivity"]:
    has_mde = row.get("minimum_detectable_paired_effect_nominal") is not None
    separation = row["power_at_complete_separation_nominal"]
    if has_mde:
        assert separation >= report["target_power"]
    else:
        assert separation < report["target_power"]

print(
    "identical-response design reproduced byte-identically: "
    f"counterfactual={report['counterfactual']} "
    f"source_task_groups={report['source_task_groups']} "
    f"effective={report['effective_source_task_groups']} "
    f"rows={len(report['disagreement_sensitivity'])} "
    f"design_digest={report['design_digest'][:16]}"
)
PY
