#!/usr/bin/env bash
# Provider-free response redistribution-right evidence gate for TASK 070.
#
# The identical-response study must verify, from primary sources, that the exact
# provider response bytes may be redistributed before it can publish anything.
# This script reproduces the frozen redistribution-right evidence record and
# byte-compares it against the committed artifact.
#
# Usage: run-identical-response-redistribution-right.sh
#
# Needs no provider, no network, and no API key. The evidence record quotes the
# frozen primary-source assignment clauses and their recorded conditions.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)

record="$repo_root/eval/governance/identical-response-redistribution-right-v1.json"
[[ -f $record ]] || { echo "missing frozen artifact: $record" >&2; exit 2; }

tmp_bin=$(mktemp /tmp/evalwitness-identical-response-redistribution-bin-XXXXXXXX)
tmp_record=$(mktemp /tmp/evalwitness-identical-response-redistribution-XXXXXXXX)
cleanup() { rm -f "$tmp_bin" "$tmp_record"; }
trap cleanup EXIT

go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" study identical-response-redistribution-right > "$tmp_record"

if ! diff -u "$record" "$tmp_record" > /dev/null; then
    echo "identical-response redistribution-right record drifted from the committed artifact" >&2
    diff -u "$record" "$tmp_record" >&2 || true
    exit 1
fi

python3 - "$record" <<'PY'
import json
import pathlib
import sys

record_path = sys.argv[1]
record = json.loads(pathlib.Path(record_path).read_text(encoding="utf-8"))

assert record["schema_version"] == "evalwitness.identical-response-redistribution-right.v1"
assert record["digest"]
assert len(record["evidence"]) == 2
assert record["summary"]["routes"] == 2
assert record["summary"]["output_rights_assigned"] == 2

for evidence in record["evidence"]:
    # Each route must quote the primary-source assignment language, record a
    # retrieval date, source URL, and scope, and decline to invent an SPDX
    # expression for a terms-of-service assignment.
    assert "assign" in evidence["assignment_clause"].lower()
    assert evidence["primary_source_url"].startswith("https://")
    assert evidence["retrieval_date"] == "2026-08-18"
    assert evidence["spdx_expression"] == "NOASSERTION"
    assert evidence["permitted_uses"]
    assert evidence["conditions"]
    assert evidence["verdict"] in {
        "output_rights_assigned_with_conditions",
        "output_rights_assigned_subject_to_internal_use_review",
    }

# The record is evidence, not authorization: it must never claim to authorize
# live calls, weight redistribution, or upstream trajectory redistribution.
joined = " ".join(record["summary"]["unsupported_claims"]).lower()
assert "live provider calls" in joined
assert "model weights" in joined
assert "trajectory" in joined

print(
    "identical-response redistribution-right record reproduced byte-identically: "
    f"routes={record['summary']['routes']} "
    f"output_rights_assigned={record['summary']['output_rights_assigned']} "
    f"digest={record['digest'][:16]}"
)
PY
