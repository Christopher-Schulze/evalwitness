#!/usr/bin/env bash
# Provider-free identical-response study protocol gate for TASK 070.
#
# The protocol states the counterfactual, primary endpoint, aggregation,
# missingness, multiplicity, stopping, minimum support, and exact uncertainty
# procedure before any provider response exists, and binds the four prior
# frozen artifacts by digest. This script reproduces the protocol and
# byte-compares it against the committed artifact.
#
# Usage: run-identical-response-protocol.sh
#
# Needs no provider, no network, and no API key.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)

protocol="$repo_root/eval/governance/identical-response-study-protocol-v1.json"
[[ -f $protocol ]] || { echo "missing frozen artifact: $protocol" >&2; exit 2; }

tmp_bin=$(mktemp /tmp/evalwitness-identical-response-protocol-bin-XXXXXXXX)
tmp_protocol=$(mktemp /tmp/evalwitness-identical-response-protocol-XXXXXXXX)
cleanup() { rm -f "$tmp_bin" "$tmp_protocol"; }
trap cleanup EXIT

go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" study identical-response-protocol --root "$repo_root" > "$tmp_protocol"

if ! diff -u "$protocol" "$tmp_protocol" > /dev/null; then
    echo "identical-response study protocol drifted from the committed artifact" >&2
    diff -u "$protocol" "$tmp_protocol" >&2 || true
    exit 1
fi

python3 - "$protocol" "$repo_root" <<'PY'
import json
import pathlib
import sys

protocol_path, repo_root = sys.argv[1], sys.argv[2]
protocol = json.loads(pathlib.Path(protocol_path).read_text(encoding="utf-8"))

assert protocol["schema_version"] == "evalwitness.identical-response-study-protocol.v1"
assert protocol["counterfactual"] == "distribution_aware_vs_chosen_token"
assert protocol["task_group_unit"] == "source_task_group"
assert protocol["minimum_task_groups"] == 40
assert protocol["eligible_task_groups"] >= 40
assert protocol["multiplicity_family"] == ["paired_task_group_disagreement"]
assert protocol["digest"]

# The protocol must bind the four prior frozen artifacts by digest, so any
# drift in design, inventory, or redistribution evidence changes its digest.
design_report = json.loads(pathlib.Path(f"{repo_root}/eval/governance/identical-response-design-report-v1.json").read_text(encoding="utf-8"))
inventory = json.loads(pathlib.Path(f"{repo_root}/eval/governance/identical-response-eligible-inventory-v1.json").read_text(encoding="utf-8"))
redist = json.loads(pathlib.Path(f"{repo_root}/eval/governance/identical-response-redistribution-right-v1.json").read_text(encoding="utf-8"))
assert protocol["design_report_digest"] == design_report["design_digest"]
assert protocol["inventory_digest"] == inventory["digest"]
assert protocol["redistribution_right_digest"] == redist["digest"]
assert protocol["eligible_task_groups"] == inventory["eligible_task_groups"]

# The protocol makes a narrow extraction-semantics claim and never a
# correctness, calibration, quality, transfer, or population claim.
for phrase in ("correctness", "calibration", "model quality", "provider transfer", "population"):
    assert any(phrase in c for c in protocol["unsupported_claims"])

print(
    "identical-response study protocol reproduced byte-identically: "
    f"counterfactual={protocol['counterfactual']} "
    f"eligible_task_groups={protocol['eligible_task_groups']} "
    f"digest={protocol['digest'][:16]}"
)
PY
