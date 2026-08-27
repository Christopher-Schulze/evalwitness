#!/usr/bin/env bash
# Provider-free eligible development inventory freeze for TASK 070.
#
# The identical-response study needs its source-task-group population frozen
# before any provider response is observed. This script derives the eligible
# inventory from the committed governed controlled-corruption release and
# byte-compares it against the frozen inventory artifact.
#
# Usage: run-identical-response-inventory.sh
#
# Needs no provider, no network, and no API key. The inventory is a
# deterministic projection over the committed release; no Monte Carlo
# randomness or live access is involved.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)

release="$repo_root/eval/governance/controlled-corruption-v3-release.json"
inventory="$repo_root/eval/governance/identical-response-eligible-inventory-v1.json"

for path in "$release" "$inventory"; do
    [[ -f $path ]] || { echo "missing frozen artifact: $path" >&2; exit 2; }
done

tmp_bin=$(mktemp /tmp/evalwitness-identical-response-inventory-bin-XXXXXXXX)
tmp_inventory=$(mktemp /tmp/evalwitness-identical-response-inventory-XXXXXXXX)
cleanup() { rm -f "$tmp_bin" "$tmp_inventory"; }
trap cleanup EXIT

go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" study identical-response-inventory --root "$repo_root" > "$tmp_inventory"

if ! diff -u "$inventory" "$tmp_inventory" > /dev/null; then
    echo "identical-response eligible inventory drifted from the committed artifact" >&2
    diff -u "$inventory" "$tmp_inventory" >&2 || true
    exit 1
fi

python3 - "$release" "$inventory" <<'PY'
import hashlib
import json
import pathlib
import sys

release_path, inventory_path = sys.argv[1], sys.argv[2]
release = json.loads(pathlib.Path(release_path).read_text(encoding="utf-8"))
inventory = json.loads(pathlib.Path(inventory_path).read_text(encoding="utf-8"))

assert inventory["schema_version"] == "evalwitness.identical-response-eligible-inventory.v1"
assert inventory["source_release_digest"] == release["digest"]
assert inventory["minimum_task_groups"] == 40
assert inventory["eligible_task_groups"] >= 40
assert inventory["eligible_task_groups"] == len(inventory["groups"])
assert inventory["digest"]

# The inventory must honestly decline redistribution: every governed source is
# reference_only, so public trajectory-body redistribution is never implied.
assert inventory["redistribution_authorized"] is False
assert {r["id"]: r["count"] for r in inventory["redistribution_classes"]} == {"reference_only": 100}

# Frozen denominators: 100 unique source task groups split 60/20/20 and
# carrying Apache-2.0 (Terminal-Bench) and MIT (SWE-bench) licenses.
role_counts = {r["id"]: r["count"] for r in inventory["task_groups_by_role"]}
assert role_counts == {"calibration": 20, "development": 60, "test": 20}
license_counts = {r["id"]: r["count"] for r in inventory["licenses"]}
assert license_counts == {"Apache-2.0": 60, "MIT": 40}

# Every admitted unit carries a license, redistribution class, task-group
# identity, source/trajectory/evidence digests, expected evidence role, and a
# contamination/data-role label. No group may repeat and all digests must be
# SHA-256 hex.
seen = set()
sha = lambda v: len(v) == 64 and all(c in "0123456789abcdef" for c in v)
for group in inventory["groups"]:
    assert group["task_group_id"].startswith("group-") and sha(group["task_group_id"][6:])
    assert group["task_group_id"] not in seen
    seen.add(group["task_group_id"])
    assert group["data_role"] in {"development", "calibration", "test"}
    assert group["redistribution_class"] == "reference_only"
    assert group["license_spdx"] in {"Apache-2.0", "MIT"}
    assert group["outcome_available"] is True
    assert len(group["source_families"]) >= 1
    assert len(group["source_digests"]) == 2 and all(sha(d) for d in group["source_digests"])
    assert len(group["trajectory_digests"]) == 2 and all(sha(d) for d in group["trajectory_digests"])
    assert len(group["outcome_witness_digests"]) == 2 and all(sha(d) for d in group["outcome_witness_digests"])
    assert group["near_duplicate_id"].startswith("near-") and sha(group["near_duplicate_id"][5:])
    assert group["lineage_cluster_id"].startswith("lineage-") and sha(group["lineage_cluster_id"][8:])

# The inventory must make zero provider calls and permit zero live access.
assert inventory["claim_boundary"]["provider_calls"] == 0
assert inventory["claim_boundary"]["agent_launches"] == 0
assert inventory["claim_boundary"]["live_response_access"] is False
assert "reference-only" in inventory["claim_boundary"]["supported_claim"]

print(
    "identical-response eligible inventory reproduced byte-identically: "
    f"eligible_task_groups={inventory['eligible_task_groups']} "
    f"roles={role_counts} "
    f"redistribution_authorized={inventory['redistribution_authorized']} "
    f"digest={inventory['digest'][:16]}"
)
PY
