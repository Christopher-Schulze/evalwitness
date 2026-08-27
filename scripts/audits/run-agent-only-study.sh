#!/usr/bin/env bash
# Provider-free, coding-agent-only formal relation study and reproducibility gate.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
tmp_dir=$(mktemp -d /tmp/evalwitness-agent-only.XXXXXX)
tmp_bin=$(mktemp /tmp/evalwitness-agent-only-bin.XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

for suffix in first second; do
  "$tmp_bin" agent-study build \
    --plan @eval/governance/controlled-corruption-v3-plan.json \
    --audit @eval/governance/controlled-corruption-v3-natural-audit.json \
    --release @eval/governance/controlled-corruption-v3-release.json \
    --calibration 20 --test 20 \
    --seed evalwitness-agent-only-controlled-relation-v1 \
    > "$tmp_dir/study-$suffix.json"
done

cmp "$tmp_dir/study-first.json" "$tmp_dir/study-second.json"
cmp "$tmp_dir/study-first.json" eval/governance/agent-only-study-v1.json
"$tmp_bin" agent-study schema > "$tmp_dir/schema.json"
cmp "$tmp_dir/schema.json" eval/governance/agent-only-study-schema-v1.json
"$tmp_bin" agent-study validate \
  --study @eval/governance/agent-only-study-v1.json \
  --plan @eval/governance/controlled-corruption-v3-plan.json \
  --audit @eval/governance/controlled-corruption-v3-natural-audit.json \
  --release @eval/governance/controlled-corruption-v3-release.json \
  > "$tmp_dir/validation.json"

python3 - "$tmp_dir/study-first.json" "$tmp_dir/validation.json" <<'PY'
import hashlib
import json
import pathlib
import sys

artifact_path = pathlib.Path("eval/governance/agent-only-study-v1.json")
artifact = json.loads(artifact_path.read_text(encoding="utf-8"))
reproduced = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
validation = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
if artifact != reproduced:
    raise SystemExit("agent-only study is not byte-identical to a fresh build")
if artifact["schema_version"] != "evalwitness.agent-only-study.v1" or artifact["canonical_policy"] != "evalwitness.agent-only-canonical-json.v1":
    raise SystemExit("agent-only study schema drifted")
if not artifact["agent_only"] or artifact["provider_calls"] != 0 or artifact["human_reviewers"] != 0:
    raise SystemExit("agent-only boundary was weakened")
if artifact["counts"] != {
    "cases": 40, "calibration": 20, "test": 20, "accepted": 40,
    "rejected": 0, "unresolved": 0, "primary_agreements": 40, "tie_breaks": 0,
}:
    raise SystemExit("agent-only result counts changed")
if artifact["selection"]["algorithm"] != "family-quota-hash-order.v1" or artifact["selection"]["calibration_chosen"] != 20 or artifact["selection"]["test_chosen"] != 20:
    raise SystemExit("agent-only selection contract changed")
if artifact["selection"]["cross_split_group_overlap"] != 0:
    raise SystemExit("agent-only calibration/test lineage overlap appeared")
if any("reviewer" in key.lower() for key in artifact if key not in {"human_reviewers"}):
    raise SystemExit("human reviewer field leaked into agent-only artifact")
if validation["digest"] != artifact["digest"] or validation["counts"] != artifact["counts"]:
    raise SystemExit("agent-only validation summary does not bind artifact")
expected_sha = hashlib.sha256(artifact_path.read_bytes()).hexdigest()
print(f"agent-only study verified: digest={artifact['digest']} sha256={expected_sha} cases=40 calibration=20 test=20")
PY

go test ./internal/agentstudy -count=1

echo "Provider calls: 0. Human reviewers: 0. Scope: formal frozen controlled-relation validity only."
